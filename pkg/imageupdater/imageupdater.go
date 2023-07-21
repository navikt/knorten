package imageupdater

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
)

type client struct {
	repo *database.Repo
	log  *logrus.Entry
}

func NewClient(repo *database.Repo, log *logrus.Entry) *client {
	return &client{
		repo: repo,
		log:  log,
	}
}

func (c *client) Run(frequency time.Duration) {
	ctx := context.Background()

	ticker := time.NewTicker(frequency)
	defer ticker.Stop()
	for {
		c.run(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *client) run(ctx context.Context) {
	if err := c.updateJupyterhubImages(ctx); err != nil {
		c.log.WithError(err).Error("updating jupyterhub images")
	}

	if err := c.updateAirflowImages(ctx); err != nil {
		c.log.WithError(err).Error("updating airflow images")
	}
}

func (c *client) triggerSync(ctx context.Context, chartType gensql.ChartType) error {
	teams, err := c.repo.TeamsForAppGet(ctx, chartType)
	if err != nil {
		return err
	}

	for _, team := range teams {
		err := c.syncChart(ctx, team[:len(team)-5], chartType)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *client) syncChart(ctx context.Context, team string, chartType gensql.ChartType) error {
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		values := chart.JupyterConfigurableValues{
			Slug: team,
		}
		return c.repo.RegisterUpdateJupyterEvent(ctx, values)
	case gensql.ChartTypeAirflow:
		values := chart.AirflowConfigurableValues{
			Slug: team,
		}
		return c.repo.RegisterUpdateAirflowEvent(ctx, values)
	}

	return nil
}

type garImage struct {
	Name string `json:"package"`
	Tag  string `json:"tags"`
}

func getLatestImageInGAR(image, tagsFilter string) (*garImage, error) {
	cmd := exec.Command(
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

	if tagsFilter != "" {
		cmd.Args = append(cmd.Args, fmt.Sprintf("--filter=TAGS:%v", tagsFilter))
	}

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return nil, err
	}

	var images []*garImage
	if err := json.Unmarshal(buf.Bytes(), &images); err != nil {
		return nil, err
	}

	return images[0], nil
}
