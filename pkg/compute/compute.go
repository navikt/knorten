package compute

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type Client struct {
	repo       *database.Repo
	gcpProject string
	gcpZone    string
	dryRun     bool
}

func NewClient(repo *database.Repo, gcpProject, gcpZone string, dryRun bool) *Client {
	return &Client{
		repo:       repo,
		gcpProject: gcpProject,
		gcpZone:    gcpZone,
		dryRun:     dryRun,
	}
}

func (c Client) Create(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool {
	log = log.WithField("owner", instance.Name)
	_, err := c.repo.ComputeInstanceGet(ctx, instance.Email)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.WithError(err).Errorf("failed retrieving compute instance %v", instance.Email)
			return true
		}
	}

	log.Infof("Creating compute instance %v", instance.Name)
	err = c.createComputeInstanceInGCP(ctx, instance.Name, instance.Email)
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
		return true
	}

	if err = c.repo.ComputeInstanceDelete(ctx, email); err != nil {
		log.WithError(err).Error("failed deleting compute instance from database")
		return true
	}

	log.Info("Successfully deleted compute instance")
	return false
}
