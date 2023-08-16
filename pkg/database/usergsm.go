package database

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) UserGSMCreate(ctx context.Context, googleSecretManager gensql.UserGoogleSecretManager) error {
	return r.querier.UserGoogleSecretManagerCreate(ctx, gensql.UserGoogleSecretManagerCreateParams(googleSecretManager))
}

func (r *Repo) UserGSMGet(ctx context.Context, owner string) (gensql.UserGoogleSecretManager, error) {
	return r.querier.UserGoogleSecretManagerGet(ctx, owner)
}

func (r *Repo) UserGSMDelete(ctx context.Context, owner string) error {
	return r.querier.UserGoogleSecretManagerDelete(ctx, owner)
}
