package database

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) ComputeInstanceCreate(ctx context.Context, instance gensql.ComputeInstance) error {
	return r.querier.ComputeInstanceCreate(ctx, gensql.ComputeInstanceCreateParams(instance))
}

func (r *Repo) ComputeInstanceUpdate(ctx context.Context, owner, diskSize string) error {
	return r.querier.ComputeInstanceUpdate(ctx, gensql.ComputeInstanceUpdateParams{
		DiskSize: diskSize,
		Owner:    owner,
	})
}

func (r *Repo) ComputeInstancesGet(ctx context.Context) ([]gensql.ComputeInstance, error) {
	return r.querier.ComputeInstancesGet(ctx)
}

func (r *Repo) ComputeInstanceGet(ctx context.Context, owner string) (gensql.ComputeInstance, error) {
	return r.querier.ComputeInstanceGet(ctx, owner)
}

func (r *Repo) ComputeInstanceDelete(ctx context.Context, owner string) error {
	return r.querier.ComputeInstanceDelete(ctx, owner)
}
