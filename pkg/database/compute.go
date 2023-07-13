package database

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) ComputeInstanceCreate(ctx context.Context, teamID, instance, machineType string) error {
	return r.querier.ComputeInstanceCreate(ctx, gensql.ComputeInstanceCreateParams{
		TeamID:       teamID,
		InstanceName: instance,
		MachineType:  gensql.ComputeMachineType(machineType),
	})
}

func (r *Repo) ComputeInstanceGet(ctx context.Context, teamID string) (gensql.ComputeInstance, error) {
	return r.querier.ComputeInstanceGet(ctx, teamID)
}

func (r *Repo) ComputeInstancesGet(ctx context.Context) ([]gensql.ComputeInstance, error) {
	return r.querier.ComputeInstancesGet(ctx)
}

func (r *Repo) SupportedComputeMachineTypes(ctx context.Context) ([]string, error) {
	return r.querier.SupportedComputeMachineTypes(ctx)
}

func (r *Repo) ComputeInstanceDelete(ctx context.Context, teamID string) error {
	return r.querier.ComputeInstanceDelete(ctx, teamID)
}
