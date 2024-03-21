package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/navikt/knorten/pkg/database/gensql"
)

func (c Client) CreateComputeInstance(ctx context.Context, instance *gensql.ComputeInstance) error {
	existingInstance, err := c.repo.ComputeInstanceGet(ctx, instance.Owner)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("retrieving compute instance: %w", err)
	}

	if existingInstance.Name != "" {
		return nil
	}

	err = c.createComputeInstanceInGCP(ctx, instance.Name, instance.Owner)
	if err != nil {
		return fmt.Errorf("creating compute instance in GCP: %w", err)
	}

	err = c.createIAMPolicyBindingsInGCP(ctx, instance.Name, instance.Owner)
	if err != nil {
		return fmt.Errorf("creating IAM policy binding: %w", err)
	}

	if err := c.repo.ComputeInstanceCreate(ctx, instance); err != nil {
		return fmt.Errorf("saving compute instance to database: %w", err)
	}

	return nil
}

func (c Client) ResizeComputeInstanceDisk(ctx context.Context, instance *gensql.ComputeInstance) error {
	err := c.resizeComputeInstanceDiskGCP(ctx, instance.Name, instance.DiskSize)
	if err != nil {
		return err
	}

	err = c.repo.ComputeInstanceUpdate(ctx, instance.Owner, instance.DiskSize)
	if err != nil {
		return err
	}

	return nil
}

func (c Client) DeleteComputeInstance(ctx context.Context, email string) error {
	instance, err := c.repo.ComputeInstanceGet(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}

		return fmt.Errorf("retrieving compute instance: %w", err)
	}

	err = c.deleteComputeInstanceFromGCP(ctx, instance.Name)
	if err != nil {
		return fmt.Errorf("deleting compute instance: %w", err)
	}

	err = c.deleteIAMPolicyBindingsFromGCP(ctx, instance.Name, email)
	if err != nil {
		return fmt.Errorf("deleting IAM policy bindings: %w", err)
	}

	err = c.repo.ComputeInstanceDelete(ctx, email)
	if err != nil {
		return fmt.Errorf("deleting compute instance from database: %w", err)
	}

	return nil
}
