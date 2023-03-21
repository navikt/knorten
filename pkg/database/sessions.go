package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database/gensql"
)

func (r *Repo) SessionCreate(ctx context.Context, session *auth.Session) error {
	return r.querier.SessionCreate(ctx, gensql.SessionCreateParams{
		Name:        session.Name,
		Email:       strings.ToLower(session.Email),
		Token:       session.Token,
		AccessToken: session.AccessToken,
		Expires:     session.Expires,
		IsAdmin:     session.IsAdmin,
	})
}

func (r *Repo) SessionGet(ctx context.Context, token string) (*auth.Session, error) {
	dbSession, err := r.querier.SessionGet(ctx, token)
	if err != nil {
		return nil, err
	}

	return &auth.Session{
		Email:       dbSession.Email,
		Name:        dbSession.Name,
		AccessToken: dbSession.AccessToken,
		Token:       dbSession.Token,
		Expires:     dbSession.Expires,
		IsAdmin:     dbSession.IsAdmin,
	}, nil
}

func (r *Repo) SessionDelete(ctx context.Context, token string) error {
	err := r.querier.SessionDelete(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Info("no session exists")
			return nil
		}
		return err
	}

	return nil
}
