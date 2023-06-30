package imageupdater

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"

	"github.com/nais/knorten/pkg/database/gensql"
)

const (
	airflowImageNameKey = "images.airflow.repository"
	airflowImageTagKey  = "images.airflow.tag"

	airflowWorkerDefaultImageNameKey = "config.kubernetes_executor.worker_container_repository"
	airflowWorkerDefaultImageTagKey  = "config.kubernetes_executor.worker_container_tag"
)

func (d *ImageUpdater) updateAirflowImages(ctx context.Context) error {
	baseUpdated, err := d.updateAirflowBaseImage(ctx)
	if err != nil {
		return err
	}

	workerUpdated, err := d.updateAirflowWorkerDefaultImage(ctx)
	if err != nil {
		return err
	}

	if baseUpdated || workerUpdated {
		if err := d.triggerSync(ctx, gensql.ChartTypeAirflow); err != nil {
			return err
		}
	}

	return nil
}

func (d *ImageUpdater) updateAirflowBaseImage(ctx context.Context) (bool, error) {
	imageName, err := d.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, airflowImageNameKey)
	if err != nil {
		return false, err
	}

	imageTag, err := d.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, airflowImageTagKey)
	if err != nil {
		return false, err
	}

	garImage, err := getLatestAirflowImageInGAR(imageName.Value)
	if err != nil {
		return false, err
	}

	if imageTag.Value != garImage.Tag {
		if err := d.repo.GlobalChartValueInsert(ctx, airflowImageTagKey, garImage.Tag, false, gensql.ChartTypeAirflow); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func (d *ImageUpdater) updateAirflowWorkerDefaultImage(ctx context.Context) (bool, error) {
	imageName, err := d.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, airflowWorkerDefaultImageNameKey)
	if err != nil {
		return false, err
	}

	imageTag, err := d.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, airflowWorkerDefaultImageTagKey)
	if err != nil {
		return false, err
	}

	garImage, err := getLatestAirflowImageInGAR(imageName.Value)
	if err != nil {
		return false, err
	}

	if imageTag.Value != garImage.Tag {
		if err := d.repo.GlobalChartValueInsert(ctx, airflowWorkerDefaultImageTagKey, garImage.Tag, false, gensql.ChartTypeAirflow); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func getLatestAirflowImageInGAR(image string) (*garImage, error) {
	listCmd := exec.Command(
		"gcloud",
		"artifacts",
		"docker",
		"images",
		"list",
		image,
		"--include-tags",
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

	return images[0], nil
}
