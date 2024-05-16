package imageupdater

import (
	"context"
	"fmt"
	"strings"
	"time"

	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"github.com/navikt/knorten/pkg/chart"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
)

type client struct {
	garClient *artifactregistry.Client
	repo      *database.Repo
	log       *logrus.Entry
}

func NewClient(ctx context.Context, repo *database.Repo, log *logrus.Entry) (*client, error) {
	garClient, err := artifactregistry.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &client{
		garClient: garClient,
		repo:      repo,
		log:       log,
	}, nil
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

func (c *client) getLatestImageTagInGAR(ctx context.Context, image, tagsFilter string) (string, error) {
	imagePath, err := imageToGCPPath(image)
	if err != nil {
		return "", err
	}
	res := c.garClient.ListVersions(ctx, &artifactregistrypb.ListVersionsRequest{
		Parent:  imagePath,
		OrderBy: "update_time",
	})

	versions := []*artifactregistrypb.Version{}
	for {
		v, err := res.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", err
		}

		versions = append(versions, v)
	}

	for i := len(versions) - 1; i >= 0; i-- {
		tag, err := c.getImageWithTagFromVersion(ctx, image, versions[i], tagsFilter)
		if err == nil {
			return getTagFromImageWithTag(tag.Name), nil
		}
	}

	return "", fmt.Errorf("tag for image %v not found in GAR with tags filter %v", image, tagsFilter)
}

func (c *client) getImageWithTagFromVersion(ctx context.Context, image string, version *artifactregistrypb.Version, tagsFilter string) (*artifactregistrypb.Tag, error) {
	imagePath, err := imageToGCPPath(image)
	if err != nil {
		return nil, err
	}
	tags := c.garClient.ListTags(ctx, &artifactregistrypb.ListTagsRequest{
		Parent: imagePath,
		Filter: fmt.Sprintf(`version="%v"`, version.Name),
	})

	for {
		tag, err := tags.Next()
		if err != nil {
			return nil, err
		}

		if strings.Contains(tag.Name, tagsFilter) {
			return tag, nil
		}
	}
}

func getTagFromImageWithTag(imgWithTag string) string {
	parts := strings.Split(imgWithTag, "/")
	return parts[len(parts)-1]
}

func imageToGCPPath(image string) (string, error) {
	imageParts := strings.Split(image, "/")
	if len(imageParts) != 4 {
		return "", fmt.Errorf("invalid image name %v", image)
	}
	location := strings.TrimSuffix(imageParts[0], "-docker.pkg.dev")
	project := imageParts[1]
	repository := imageParts[2]
	imageName := imageParts[3]

	return fmt.Sprintf("projects/%v/locations/%v/repositories/%v/packages/%v", project, location, repository, imageName), nil
}
