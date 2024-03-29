package user

import (
	"context"
	"database/sql"
	"errors"

	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/logger"
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
		log.WithError(err).Infof("failed retrieving compute instance %v", instance.Owner)
		return true, err
	}

	if existingInstance.Name != "" {
		return false, nil
	}

	err = c.createComputeInstanceInGCP(ctx, instance.Name, instance.Owner)
	if err != nil {
		log.WithError(err).Info("failed creating compute instance in GCP")
		return true, err
	}

	err = c.createIAMPolicyBindingsInGCP(ctx, instance.Name, instance.Owner)
	if err != nil {
		log.WithError(err).Info("failed creating IAM policy binding")
		return true, err
	}

	if err := c.repo.ComputeInstanceCreate(ctx, instance); err != nil {
		log.WithError(err).Info("failed saving compute instance to database")
		return true, err
	}

	return false, nil
}

func (c Client) ResizeComputeInstanceDisk(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool {
	log.Info("Resizing compute instance disk")

	if err := c.resizeComputeInstanceDisk(ctx, instance, log); err != nil {
		log.Info("failed to resize compute instance disk")
		return true
	}

	log.Info("Successfully resized compute instance disk")
	return false
}

func (c Client) resizeComputeInstanceDisk(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) error {
	if err := c.resizeComputeInstanceDiskGCP(ctx, instance.Name, instance.DiskSize); err != nil {
		log.WithError(err).Info("resizing compute instance disk")
		return err
	}

	if err := c.repo.ComputeInstanceUpdate(ctx, instance.Owner, instance.DiskSize); err != nil {
		log.WithError(err).Infof("failed updating compute instance in database for owner %v", instance.Owner)
		return err
	}

	return nil
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

		log.WithError(err).Info("failed retrieving compute instance")
		return true, err
	}

	if err := c.deleteComputeInstanceFromGCP(ctx, instance.Name); err != nil {
		log.WithError(err).Info("failed deleting compute instance from GCP")
		return true, err
	}

	if err := c.deleteIAMPolicyBindingsFromGCP(ctx, instance.Name, email); err != nil {
		log.WithError(err).Info("failed deleting IAM policy binding")
		return true, err
	}

	if err = c.repo.ComputeInstanceDelete(ctx, email); err != nil {
		log.WithError(err).Info("failed deleting compute instance from database")
		return true, err
	}

	return false, nil
}
