package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"

	_ "github.com/lib/pq"

	// Pin version of sqlc cli
	_ "github.com/kyleconroy/sqlc"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Repo struct {
	querier Querier
	db      *sql.DB
	log     *logrus.Entry
}

type Querier interface {
	gensql.Querier
	WithTx(tx *sql.Tx) *gensql.Queries
}

func New(dbConnDSN string, log *logrus.Entry) (*Repo, error) {
	db, err := sql.Open("postgres", dbConnDSN)
	if err != nil {
		return nil, fmt.Errorf("open sql connection: %w", err)
	}

	goose.SetBaseFS(embedMigrations)

	if err := goose.Up(db, "migrations"); err != nil {
		return nil, fmt.Errorf("goose up: %w", err)
	}

	return &Repo{
		querier: gensql.New(db),
		db:      db,
		log:     log,
	}, nil
}

func (r *Repo) GlobalChartValueInsert(ctx context.Context, key, value string, chartType gensql.ChartType) error {
	return r.querier.GlobalValueInsert(ctx, gensql.GlobalValueInsertParams{
		Key:       key,
		Value:     value,
		ChartType: chartType,
	})
}

func (r *Repo) GlobalValuesGet(ctx context.Context, chartType gensql.ChartType) ([]gensql.ChartGlobalValue, error) {
	return r.querier.GlobalValuesGet(ctx, chartType)
}

func (r *Repo) TeamChartValueInsert(ctx context.Context, key, value, team string, chartType gensql.ChartType) error {
	return r.querier.TeamValueInsert(ctx, gensql.TeamValueInsertParams{
		Key:       key,
		Value:     value,
		Team:      team,
		ChartType: chartType,
	})
}

func (r *Repo) TeamValuesGet(ctx context.Context, chartType gensql.ChartType, team string) ([]gensql.ChartTeamValue, error) {
	return r.querier.TeamValuesGet(ctx, gensql.TeamValuesGetParams{
		ChartType: chartType,
		Team:      team,
	})
}

func (r *Repo) UserAppInsert(ctx context.Context, email, team string, chartType gensql.ChartType) error {
	return r.querier.UserAppInsert(ctx, gensql.UserAppInsertParams{
		Email:     email,
		Team:      team,
		ChartType: chartType,
	})
}

func (r *Repo) UserAppsGet(ctx context.Context, email string) ([]gensql.UserAppsGetRow, error) {
	return r.querier.UserAppsGet(ctx, email)
}

func (r *Repo) SessionCreate(ctx context.Context, session *auth.Session) error {
	return r.querier.SessionCreate(ctx, gensql.SessionCreateParams{
		Name:        session.Name,
		Email:       strings.ToLower(session.Email),
		Token:       session.Token,
		AccessToken: session.AccessToken,
		Expires:     session.Expires,
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
