package imageupdater

import (
	"context"
	"encoding/json"
	"fmt"
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

func (c *client) updateJupyterhubImages(ctx context.Context) error {
	profilesDB, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeJupyterhub, profileListHelmKey)
	if err != nil {
		c.log.WithError(err).Error("getting jupyterhub singleuser profiles from database")
		return err
	}

	var profiles []*profile
	if err := json.Unmarshal([]byte(profilesDB.Value), &profiles); err != nil {
		c.log.WithError(err).Error("unmarshalling profiles")
		return err
	}

	var profilesStatus []bool
	for _, p := range profiles {
		updated, err := updateIfNeeded(p)
		if err != nil {
			c.log.WithError(err).Error("checking image up to date with GAR")
		}
		profilesStatus = append(profilesStatus, updated)
	}

	if hasUpdates(profilesStatus) {
		profilesBytes, err := json.Marshal(profiles)
		if err != nil {
			c.log.WithError(err).Error("marshalling updated profiles")
			return err
		}

		if err := c.repo.GlobalChartValueInsert(ctx, profileListHelmKey, string(profilesBytes), false, gensql.ChartTypeJupyterhub); err != nil {
			c.log.WithError(err).Error("inserting updated profile list in db")
			return err
		}

		if err := c.triggerSync(ctx, gensql.ChartTypeJupyterhub); err != nil {
			return err
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

	garImage, err := getLatestImageInGAR(image, pythonVersion)
	if err != nil {
		return false, err
	}

	if tag != garImage.Tag {
		p.KubespawnerOverride.Image = fmt.Sprintf("%v:%v", garImage.Name, garImage.Tag)
		return true, nil
	}

	return false, nil
}

func hasUpdates(statuses []bool) bool {
	for _, s := range statuses {
		if s {
			return true
		}
	}

	return false
}
