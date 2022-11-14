package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/postgres"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/reflect"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
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

func (r *Repo) NewSessionStore() (gin.HandlerFunc, error) {
	// TODO: Create session key
	store, err := postgres.NewStore(r.db, []byte("securecookie.GenerateRandomKey(32)"))
	if err != nil {
		return nil, err
	}

	return sessions.Sessions("session", store), nil
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

func (r *Repo) TeamConfigurableValuesGet(ctx context.Context, chartType gensql.ChartType, team string, obj any) error {
	teamValues, err := r.querier.TeamValuesGet(ctx, gensql.TeamValuesGetParams{
		ChartType: chartType,
		Team:      team,
	})
	if err != nil {
		return err
	}

	values := map[string]string{}
	for _, value := range teamValues {
		values[value.Key] = value.Value
	}

	return reflect.InterfaceToStruct(obj, values)
}
