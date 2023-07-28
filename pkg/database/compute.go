package database

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) ComputeInstanceCreate(ctx context.Context, instance gensql.ComputeInstance) error {
	return r.querier.ComputeInstanceCreate(ctx, gensql.ComputeInstanceCreateParams(instance))
}

func (r *Repo) ComputeInstanceGet(ctx context.Context, email string) (gensql.ComputeInstance, error) {
	return r.querier.ComputeInstanceGet(ctx, email)
}

func (r *Repo) ComputeInstanceDelete(ctx context.Context, email string) error {
	return r.querier.ComputeInstanceDelete(ctx, email)
}
