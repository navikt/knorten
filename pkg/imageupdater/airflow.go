package imageupdater

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
)

const (
	airflowImageNameKey = "images.airflow.repository"
	airflowImageTagKey  = "images.airflow.tag"

	airflowWorkerDefaultImageNameKey = "config.kubernetes_executor.worker_container_repository"
	airflowWorkerDefaultImageTagKey  = "config.kubernetes_executor.worker_container_tag"
)

func (c *client) updateAirflowImages(ctx context.Context) error {
	baseUpdated, err := c.updateAirflowBaseImage(ctx)
	if err != nil {
		return err
	}

	workerUpdated, err := c.updateAirflowWorkerDefaultImage(ctx)
	if err != nil {
		return err
	}

	if baseUpdated || workerUpdated {
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

func (c *client) updateAirflowWorkerDefaultImage(ctx context.Context) (bool, error) {
	imageName, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, airflowWorkerDefaultImageNameKey)
	if err != nil {
		return false, err
	}

	imageTag, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, airflowWorkerDefaultImageTagKey)
	if err != nil {
		return false, err
	}

	garImage, err := getLatestImageInGAR(imageName.Value, "")
	if err != nil {
		return false, err
	}

	if imageTag.Value != garImage.Tag {
		if err := c.repo.GlobalChartValueInsert(ctx, airflowWorkerDefaultImageTagKey, garImage.Tag, false, gensql.ChartTypeAirflow); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}
