package imageupdater

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/navikt/knorten/pkg/chart"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
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
	teams, err := c.repo.TeamsForChartGet(ctx, chartType)
	if err != nil {
		return err
	}

	for _, team := range teams {
		err := c.syncChart(ctx, team, chartType)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *client) syncChart(ctx context.Context, teamID string, chartType gensql.ChartType) error {
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		values := chart.JupyterConfigurableValues{
			TeamID: teamID,
		}
		return c.repo.RegisterUpdateJupyterEvent(ctx, teamID, values)
	case gensql.ChartTypeAirflow:
		values := chart.AirflowConfigurableValues{
			TeamID: teamID,
		}
		return c.repo.RegisterUpdateAirflowEvent(ctx, teamID, values)
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
		"--quiet",
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

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	var images []*garImage
	if err := json.Unmarshal(stdOut.Bytes(), &images); err != nil {
		return nil, err
	}

	return images[0], nil
}
