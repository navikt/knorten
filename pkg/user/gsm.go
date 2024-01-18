package user

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

func (c Client) CreateUserGSM(ctx context.Context, manager gensql.UserGoogleSecretManager, log logger.Logger) bool {
	log.Info("Creating User Google Secret Manager")

	if retry, err := c.createGSM(ctx, manager, log); err != nil {
		log.Info("failed creating User Google Secret Manager")
		return retry
	}

	log.Info("Successfully created User Google Secret Manager")
	return false
}

func (c Client) createGSM(ctx context.Context, manager gensql.UserGoogleSecretManager, log logger.Logger) (bool, error) {
	existingInstance, err := c.repo.UserGSMGet(ctx, manager.Owner)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.WithError(err).Infof("failed retrieving User Google Secret Manager %v", manager.Owner)
		return true, err
	}

	if existingInstance.Name != "" {
		return false, nil
	}

	err = c.createUserGSMInGCP(ctx, manager.Name, manager.Owner)
	if err != nil {
		log.WithError(err).Info("failed creating User Google Secret Manager in GCP")
		return true, err
	}

	if err := c.repo.UserGSMCreate(ctx, manager); err != nil {
		log.WithError(err).Info("failed saving User Google Secret Manager to database")
		return true, err
	}

	return false, nil
}

func (c Client) DeleteUserGSM(ctx context.Context, email string, log logger.Logger) bool {
	log.Info("Deleting User Google Secret Manager")

	if retry, err := c.deleteGSM(ctx, email, log); err != nil {
		log.Info("failed creating User Google Secret Manager")
		return retry
	}

	log.Info("Successfully deleted User Google Secret Manager")
	return false
}

func (c Client) deleteGSM(ctx context.Context, email string, log logger.Logger) (bool, error) {
	instance, err := c.repo.UserGSMGet(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		log.WithError(err).Info("failed retrieving User Google Secret Manager")
		return true, err
	}

	if err := c.deleteUserGSMFromGCP(ctx, instance.Name); err != nil {
		log.WithError(err).Info("failed deleting User Google Secret Manager from GCP")
		return true, err
	}

	if err = c.repo.UserGSMDelete(ctx, email); err != nil {
		log.WithError(err).Info("failed deleting User Google Secret Manager from database")
		return true, err
	}

	return false, nil
}
