package imageupdater

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/nais/knorten/pkg/database/gensql"
)

const (
	profileListHelmKey = "singleuser.profileList"
)

type profile struct {
	Name                string `json:"display_name"`
	Description         string `json:"description"`
	KubespawnerOverride struct {
		Image string `json:"image"`
	} `json:"kubespawner_override"`
}

func (d *ImageUpdater) updateJupyterhubImages(ctx context.Context) error {
	profilesDB, err := d.repo.GlobalValueGet(ctx, gensql.ChartTypeJupyterhub, profileListHelmKey)
	if err != nil {
		d.log.WithError(err).Error("getting jupyterhub singleuser profiles from database")
		return err
	}

	profiles := []*profile{}
	if err := json.Unmarshal([]byte(profilesDB.Value), &profiles); err != nil {
		d.log.WithError(err).Error("unmarshalling profiles")
		return err
	}

	profilesStatus := []bool{}
	for _, p := range profiles {
		updated, err := updateIfNeeded(p)
		if err != nil {
			d.log.WithError(err).Error("checking image up to date with GAR")
		}
		profilesStatus = append(profilesStatus, updated)
	}

	if hasUpdates(profilesStatus) {
		profilesBytes, err := json.Marshal(profiles)
		if err != nil {
			d.log.WithError(err).Error("marshalling updated profiles")
			return err
		}

		if err := d.repo.GlobalChartValueInsert(ctx, profileListHelmKey, string(profilesBytes), false, gensql.ChartTypeJupyterhub); err != nil {
			d.log.WithError(err).Error("inserting updated profile list in db")
			return err
		}

		teams, err := d.repo.TeamsForAppGet(ctx, gensql.ChartTypeJupyterhub)
		if err != nil {
			d.log.WithError(err).Error("reading jupyterhub teams from db")
			return err
		}

		for _, t := range teams {
			if err := d.jupyterClient.Sync(ctx, t); err != nil {
				d.log.WithError(err).Errorf("error syncing jupyterhub for team %v", t)
				return err
			}
		}
	}

	return nil
}

func updateIfNeeded(p *profile) (bool, error) {
	parts := strings.Split(p.KubespawnerOverride.Image, ":")
	if len(parts) != 2 {
		return false, fmt.Errorf("image format invalid, should be image:tag, got %v", p.KubespawnerOverride.Image)
	}
	image := parts[0]
	tag := parts[1]

	tagParts := strings.Split(tag, "-")
	if len(tagParts) != 5 {
		return false, fmt.Errorf("tag format invalid, should be yyyy-mm-dd-gitsha-pythonVersion, got %v", tag)
	}
	pythonVersion := tagParts[4]

	garImage, err := getLatestJupyterImageInGAR(image, pythonVersion)
	if err != nil {
		return false, err
	}

	if tag != garImage.Tag {
		p.KubespawnerOverride.Image = fmt.Sprintf("%v:%v", garImage.Name, garImage.Tag)
		return true, nil
	}

	return false, nil
}

func getLatestJupyterImageInGAR(image, pythonVersion string) (*garImage, error) {
	listCmd := exec.Command(
		"gcloud",
		"artifacts",
		"docker",
		"images",
		"list",
		image,
		"--include-tags",
		"--filter",
		fmt.Sprintf("TAGS:-%v", pythonVersion),
		"--sort-by=~Update_Time",
		"--limit=1",
		"--format=json")

	buf := &bytes.Buffer{}
	listCmd.Stdout = buf
	listCmd.Stderr = os.Stderr
	if err := listCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return nil, err
	}

	var images []*garImage
	if err := json.Unmarshal(buf.Bytes(), &images); err != nil {
		return nil, err
	}

	if len(images) != 1 {
		return nil, fmt.Errorf("gar image list command should return one (and only one) image with the filters set, received %v images", len(images))
	}

	return images[0], nil
}

func hasUpdates(statuses []bool) bool {
	for _, s := range statuses {
		if s {
			return true
		}
	}

	return false
}
