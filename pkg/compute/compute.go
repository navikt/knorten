package compute

import (
	"context"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
)

type Client struct {
	log        *logrus.Entry
	repo       *database.Repo
	gcpProject string
	dryRun     bool
}

func NewClient(repo *database.Repo, gcpProject string, dryRun bool, log *logrus.Entry) *Client {
	return &Client{
		log:        log,
		repo:       repo,
		gcpProject: gcpProject,
		dryRun:     dryRun,
	}
}

func (c Client) Create(ctx context.Context, instance gensql.ComputeInstance) bool {
	c.createComputeInstanceInGCP(ctx, instance.Name, instance.Email)

	err := c.repo.ComputeInstanceCreate(ctx, instance)
	if err != nil {
		c.log.Errorf("failed creating compute instance: %v", err)
		return true
	}

	return false
}

func (c Client) Delete(ctx context.Context, email string) bool {
	instance, err := c.repo.ComputeInstanceGet(ctx, email)
	if err != nil {
		c.log.Errorf("failed deleting compute instance: %v", err)
		return true
	}

	err = c.deleteComputeInstanceFromGCP(ctx, instance.Name)
	if err != nil {
		c.log.Errorf("failed deleting compute instance: %v", err)
		return false
	}

	err = c.repo.ComputeInstanceDelete(ctx, email)
	if err != nil {
		c.log.Errorf("failed deleting compute instance: %v", err)
		return true
	}

	return false
}
