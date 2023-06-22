package imageupdater

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/nais/knorten/pkg/database/gensql"
)

const (
	airflowImageNameKey = "images.airflow.repository"
	airflowImageTagKey  = "images.airflow.tag"
)

func (d *ImageUpdater) updateAirflowBaseImage(ctx context.Context) error {
	imageName, err := d.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, airflowImageNameKey)
	if err != nil {
		return err
	}

	imageTag, err := d.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, airflowImageTagKey)
	if err != nil {
		return err
	}

	garImage, err := getLatestAirflowImageInGAR(imageName.Value)
	if err != nil {
		return err
	}

	if imageTag.Value != garImage.Tag {
		if err := d.repo.GlobalChartValueInsert(ctx, airflowImageTagKey, garImage.Tag, false, gensql.ChartTypeAirflow); err != nil {
			return err
		}
	}

	return nil
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

	if len(images) != 1 {
		return nil, fmt.Errorf("gar image list command should return one (and only one) image with the filters set, received %v images", len(images))
	}

	return images[0], nil
}
