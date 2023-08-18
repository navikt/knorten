package user

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

func (c Client) CreateComputeInstance(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool {
	log.Info("Creating compute instance")

	if retry, err := c.createComputeInstance(ctx, instance, log); err != nil {
		log.Info("failed creating compute instance")
		return retry
	}

	log.Info("Successfully created compute instance")
	return false
}

func (c Client) createComputeInstance(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) (bool, error) {
	existingInstance, err := c.repo.ComputeInstanceGet(ctx, instance.Owner)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.WithError(err).Errorf("failed retrieving compute instance %v", instance.Owner)
		return true, err
	}

	if existingInstance.Name != "" {
		return false, nil
	}

	err = c.createComputeInstanceInGCP(ctx, instance.Name, instance.Owner)
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

func (c Client) DeleteComputeInstance(ctx context.Context, email string, log logger.Logger) bool {
	log.Info("Deleting compute instance")

	if retry, err := c.deleteComputeInstance(ctx, email, log); err != nil {
		log.Info("failed creating compute instance")
		return retry
	}

	log.Info("Successfully deleted compute instance")
	return false
}

func (c Client) deleteComputeInstance(ctx context.Context, email string, log logger.Logger) (bool, error) {
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