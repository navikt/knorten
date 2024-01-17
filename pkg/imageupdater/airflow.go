package imageupdater

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nais/knorten/pkg/database/gensql"
)

const (
	airflowBaseImagesRepositoryKey    = "images.airflow.repository"
	airflowBaseImagesTagKey           = "images.airflow.tag"
	airflowGitSyncImagesRepositoryKey = "images.gitSync.repository"
	airflowGitSyncImagesTagKey        = "images.gitSync.tag"
	airflowEnvKey                     = "env"
)

var imageEnvNames = []string{"CLONE_REPO_IMAGE", "KNADA_AIRFLOW_OPERATOR_IMAGE"}

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
		return false, err
	}

	imageTag, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, imageTagKey)
	if err != nil {
		return false, err
	}

	garImage, err := getLatestImageInGAR(imageName.Value, "")
	if err != nil {
		return false, err
	}

	if imageTag.Value != garImage.Tag {
		if err := c.repo.GlobalChartValueInsert(ctx, imageNameKey, garImage.Tag, false, gensql.ChartTypeAirflow); err != nil {
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
		return false, err
	}

	type globalEnv struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	globalEnvs := []*globalEnv{}
	if err := json.Unmarshal([]byte(globalEnvsSQL.Value), &globalEnvs); err != nil {
		return false, err
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
				return false, err
			}

			if currentImageParts[1] != latestImage.Tag {
				env.Value = fmt.Sprintf("%v:%v", currentImageParts[0], latestImage.Tag)
				globalEnvsMarshalled, err := json.Marshal(globalEnvs)
				if err != nil {
					return false, err
				}
				if err := c.repo.GlobalChartValueInsert(ctx, airflowEnvKey, string(globalEnvsMarshalled), false, gensql.ChartTypeAirflow); err != nil {
					return false, err
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
