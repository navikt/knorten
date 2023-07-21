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
	log = log.WithField("owner", instance.Name)
	log.Infof("Creating compute instance %v", instance.Name)

	err := c.createComputeInstanceInGCP(ctx, instance.Name, instance.Email)
	if err != nil {
		log.WithError(err).Error("failed creating compute instance in GCP")
		return true
	}

	if err := c.repo.ComputeInstanceCreate(ctx, instance); err != nil {
		log.WithError(err).Error("failed saving compute instance to database")
		return true
	}

	log.Infof("Successfully created compute instance %v", instance.Name)
	return false
}

func (c Client) Delete(ctx context.Context, email string, log logger.Logger) bool {
	log = log.WithField("owner", email)
	log.Info("Deleting compute instance")

	instance, err := c.repo.ComputeInstanceGet(ctx, email)
	if err != nil {
		log.WithError(err).Error("failed retrieving compute instance")
		return true
	}

	if err := c.deleteComputeInstanceFromGCP(ctx, instance.Name); err != nil {
		log.WithError(err).Error("failed deleting compute instance from GCP")
		return false
	}

	if err = c.repo.ComputeInstanceDelete(ctx, email); err != nil {
		log.WithError(err).Error("failed deleting compute instance from database")
		return true
	}

	log.Info("Successfully deleted compute instance")
	return false
}
