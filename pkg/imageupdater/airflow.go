package imageupdater

import (
	"context"
	"fmt"

	"github.com/nais/knorten/pkg/database/gensql"
)

const (
	airflowImageNameKey = "images.airflow.repository"
	airflowImageTagKey  = "images.airflow.tag"
)

func (c *client) updateAirflowImages(ctx context.Context) error {
	baseUpdated, err := c.updateAirflowBaseImage(ctx)
	if err != nil {
		return fmt.Errorf("updating airflow base image: %w", err)
	}

	if baseUpdated {
		if err := c.triggerSync(ctx, gensql.ChartTypeAirflow); err != nil {
			return err
		}
	}

	return nil
}

func (c *client) updateAirflowBaseImage(ctx context.Context) (bool, error) {
	imageName, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, airflowImageNameKey)
	if err != nil {
		return false, err
	}

	imageTag, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, airflowImageTagKey)
	if err != nil {
		return false, err
	}

	garImage, err := getLatestImageInGAR(imageName.Value, "")
	if err != nil {
		return false, err
	}

	if imageTag.Value != garImage.Tag {
		if err := c.repo.GlobalChartValueInsert(ctx, airflowImageTagKey, garImage.Tag, false, gensql.ChartTypeAirflow); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}
