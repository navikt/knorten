package compute

import (
	"context"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type Client struct {
	repo       *database.Repo
	gcpProject string
	dryRun     bool
}

func NewClient(repo *database.Repo, gcpProject string, dryRun bool) *Client {
	return &Client{
		repo:       repo,
		gcpProject: gcpProject,
		dryRun:     dryRun,
	}
}

func (c Client) Create(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool {
	err := c.createComputeInstanceInGCP(ctx, instance.Name, instance.Email)
	if err != nil {
		log.Errorf("failed creating compute instance: %v", err)
		return true
	}

	if err := c.repo.ComputeInstanceCreate(ctx, instance); err != nil {
		log.Errorf("failed saving compute instance to database: %v", err)
		return true
	}

	return false
}

func (c Client) Delete(ctx context.Context, email string, log logger.Logger) bool {
	instance, err := c.repo.ComputeInstanceGet(ctx, email)
	if err != nil {
		log.Errorf("failed deleting compute instance: %v", err)
		return true
	}

	if err := c.deleteComputeInstanceFromGCP(ctx, instance.Name); err != nil {
		log.Errorf("failed deleting compute instance: %v", err)
		return false
	}

	if err = c.repo.ComputeInstanceDelete(ctx, email); err != nil {
		log.Errorf("failed deleting compute instance: %v", err)
		return true
	}

	return false
}
