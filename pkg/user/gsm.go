package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/navikt/knorten/pkg/database/gensql"
)

func (c Client) CreateUserGSM(ctx context.Context, manager *gensql.UserGoogleSecretManager) error {
	existingInstance, err := c.repo.UserGSMGet(ctx, manager.Owner)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("retrieving User Google Secret Manager: %w", err)
	}

	if existingInstance.Name != "" {
		return nil
	}

	err = c.createUserGSMInGCP(ctx, manager.Name, manager.Owner)
	if err != nil {
		return fmt.Errorf("creating User Google Secret Manager in GCP: %w", err)
	}

	if err := c.repo.UserGSMCreate(ctx, manager); err != nil {
		return fmt.Errorf("saving User Google Secret Manager to database: %w", err)
	}

	return nil
}

func (c Client) DeleteUserGSM(ctx context.Context, email string) error {
	instance, err := c.repo.UserGSMGet(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}

		return fmt.Errorf("retrieving User Google Secret Manager: %w", err)
	}

	if err := c.deleteUserGSMFromGCP(ctx, instance.Name); err != nil {
		return fmt.Errorf("deleting User Google Secret Manager from GCP: %w", err)
	}

	if err = c.repo.UserGSMDelete(ctx, email); err != nil {
		return fmt.Errorf("deleting User Google Secret Manager from database: %w", err)
	}

	return nil
}
