package compute

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type Client interface {
	Create(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool
	Delete(ctx context.Context, email string, log logger.Logger) bool
}

type ComputeClient struct {
	repo       *database.Repo
	gcpProject string
	gcpZone    string
	dryRun     bool
}

func NewClient(repo *database.Repo, gcpProject, gcpZone string, dryRun bool) *ComputeClient {
	return &ComputeClient{
		repo:       repo,
		gcpProject: gcpProject,
		gcpZone:    gcpZone,
		dryRun:     dryRun,
	}
}

func (c ComputeClient) Create(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool {
	log = log.WithField("owner", instance.Name)
	log.Info("Creating compute instance")

	if retry, err := c.create(ctx, instance, log); err != nil {
		log.Info("failed creating compute instance")
		return retry
	}

	log.Info("Successfully created compute instance")
	return false
}

func (c ComputeClient) create(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) (bool, error) {
	existingInstance, err := c.repo.ComputeInstanceGet(ctx, instance.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.WithError(err).Errorf("failed retrieving compute instance %v", instance.Email)
		return true, err
	}

	if existingInstance.Name != "" {
		return false, nil
	}

	err = c.createComputeInstanceInGCP(ctx, instance.Name, instance.Email)
	if err != nil {
		log.WithError(err).Error("failed creating compute instance in GCP")
		return true, err
	}

	if err := c.repo.ComputeInstanceCreate(ctx, instance); err != nil {
		log.WithError(err).Error("failed saving compute instance to database")
		return true, err
	}

	return false, nil
}

func (c ComputeClient) Delete(ctx context.Context, email string, log logger.Logger) bool {
	log = log.WithField("owner", email)
	log.Info("Deleting compute instance")

	if retry, err := c.delete(ctx, email, log); err != nil {
		log.Info("failed creating compute instance")
		return retry
	}

	log.Info("Successfully deleted compute instance")
	return false
}

func (c ComputeClient) delete(ctx context.Context, email string, log logger.Logger) (bool, error) {
	instance, err := c.repo.ComputeInstanceGet(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		log.WithError(err).Error("failed retrieving compute instance")
		return true, err
	}

	if err := c.deleteComputeInstanceFromGCP(ctx, instance.Name); err != nil {
		log.WithError(err).Error("failed deleting compute instance from GCP")
		return true, err
	}

	if err = c.repo.ComputeInstanceDelete(ctx, email); err != nil {
		log.WithError(err).Error("failed deleting compute instance from database")
		return true, err
	}

	return false, nil
}
