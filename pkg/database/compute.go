package database

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) ComputeInstanceCreate(ctx context.Context, instance gensql.ComputeInstance) error {
	return r.querier.ComputeInstanceCreate(ctx, gensql.ComputeInstanceCreateParams(instance))
}

func (r *Repo) ComputeInstanceGet(ctx context.Context, owner string) (gensql.ComputeInstance, error) {
	return r.querier.ComputeInstanceGet(ctx, owner)
}

func (r *Repo) ComputeInstanceDelete(ctx context.Context, owner string) error {
	return r.querier.ComputeInstanceDelete(ctx, owner)
}
