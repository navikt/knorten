package imageupdater

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/navikt/knorten/pkg/database/gensql"
)

const (
	airflowBaseImagesRepositoryKey    = "images.airflow.repository"
	airflowBaseImagesTagKey           = "images.airflow.tag"
	airflowGitSyncImagesRepositoryKey = "images.gitSync.repository"
	airflowGitSyncImagesTagKey        = "images.gitSync.tag"
	airflowEnvKey                     = "env"
)

var imageEnvNames = []string{
	"CLONE_REPO_IMAGE",
	"KNADA_AIRFLOW_OPERATOR_IMAGE",
	"DATAVERK_IMAGE_PYTHON_38",
	"DATAVERK_IMAGE_PYTHON_39",
	"DATAVERK_IMAGE_PYTHON_310",
	"DATAVERK_IMAGE_PYTHON_311",
	"DATAVERK_IMAGE_PYTHON_312",
}

func (c *client) updateAirflowImages(ctx context.Context) error {
	baseImageUpdated, err := c.updateAirflowImage(ctx, airflowBaseImagesRepositoryKey, airflowBaseImagesTagKey)
	if err != nil {
		return fmt.Errorf("updating airflow base image: %w", err)
	}

	syncImageUpdated, err := c.updateAirflowImage(ctx, airflowGitSyncImagesRepositoryKey, airflowGitSyncImagesTagKey)
	if err != nil {
		return fmt.Errorf("updating airflow git sync image: %w", err)
	}

	globalEnvsUpdated, err := c.updateGlobalEnvs(ctx)
	if err != nil {
		return fmt.Errorf("updating airflow global envs: %w", err)
	}

	if baseImageUpdated || syncImageUpdated || globalEnvsUpdated {
		if err := c.triggerSync(ctx, gensql.ChartTypeAirflow); err != nil {
			return err
		}
	}

	return nil
}

func (c *client) updateAirflowImage(ctx context.Context, imageNameKey, imageTagKey string) (bool, error) {
	imageName, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, imageNameKey)
	if err != nil {
		return false, fmt.Errorf("getting image name: %w", err)
	}

	imageTag, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, imageTagKey)
	if err != nil {
		return false, fmt.Errorf("getting image tag: %w", err)
	}

	// Skip updating the image if it's the apache/airflow image
	if imageName.Value == "apache/airflow" {
		return false, nil
	}

	garImage, err := getLatestImageInGAR(imageName.Value, "")
	if err != nil {
		return false, fmt.Errorf("getting latest image in GAR: %w, image: %s", err, imageName.Value)
	}

	if imageTag.Value != garImage.Tag {
		if err := c.repo.GlobalChartValueInsert(ctx, imageTagKey, garImage.Tag, false, gensql.ChartTypeAirflow); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func (c *client) updateGlobalEnvs(ctx context.Context) (bool, error) {
	globalEnvsSQL, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, airflowEnvKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, fmt.Errorf("getting global envs: %w", err)
	}

	type globalEnv struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	var globalEnvs []*globalEnv
	if err := json.Unmarshal([]byte(globalEnvsSQL.Value), &globalEnvs); err != nil {
		return false, fmt.Errorf("unmarshalling global envs: %w", err)
	}

	globalEnvsUpdated := false
	for _, env := range globalEnvs {
		if contains(imageEnvNames, env.Name) {
			currentImageParts := strings.Split(env.Value, ":")
			if len(currentImageParts) != 2 {
				return false, fmt.Errorf("invalid image format for image %v, should be <image>:<tag>", env.Value)
			}

			latestImage, err := getLatestImageInGAR(currentImageParts[0], "")
			if err != nil {
				return false, fmt.Errorf("getting latest image in GAR: %w, image: %s", err, currentImageParts[0])
			}

			if currentImageParts[1] != latestImage.Tag {
				env.Value = fmt.Sprintf("%v:%v", currentImageParts[0], latestImage.Tag)
				globalEnvsMarshalled, err := json.Marshal(globalEnvs)
				if err != nil {
					return false, fmt.Errorf("marshalling global envs: %w", err)
				}
				if err := c.repo.GlobalChartValueInsert(ctx, airflowEnvKey, string(globalEnvsMarshalled), false, gensql.ChartTypeAirflow); err != nil {
					return false, fmt.Errorf("updating global envs: %w", err)
				}
				globalEnvsUpdated = true
			}
		}
	}

	return globalEnvsUpdated, nil
}

func contains(envs []string, name string) bool {
	for _, e := range envs {
		if e == name {
			return true
		}
	}
	return false
}
