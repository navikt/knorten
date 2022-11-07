package database

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
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

type JupyterConfigurableValues struct {
	AdminUsers      []string `form:"users[]" binding:"required" helm:"hub.config.Authenticator.admin_users"`
	AllowedUsers    []string `form:"users[]" binding:"required" helm:"hub.config.Authenticator.allowed_users"`
	CPULimit        string   `form:"cpu" helm:"singleuser.cpu.limit"`
	CPUGuarantee    string   `form:"cpu" helm:"singleuser.cpu.guarantee"`
	MemoryLimit     string   `form:"memory" helm:"singleuser.memory.limit"`
	MemoryGuarantee string   `form:"memory" helm:"singleuser.memory.guarantee"`
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

func (r *Repo) TeamConfigurableValuesGet(ctx context.Context, chartType gensql.ChartType, team string) (JupyterConfigurableValues, error) {
	teamValues, err := r.querier.TeamValuesGet(ctx, gensql.TeamValuesGetParams{
		ChartType: chartType,
		Team:      team,
	})

	var configurableValues JupyterConfigurableValues
	for i, value := range teamValues {
		fmt.Println(i, value)
		switch value.Key {
		case "singleuser.cpu.limit":
			configurableValues.CPULimit = value.Value
		case "singleuser.cpu.guarantee":
			configurableValues.CPUGuarantee = value.Value
		case "singleuser.memory.limit":
			configurableValues.MemoryLimit = value.Value
		case "singleuser.memory.guarantee":
			configurableValues.MemoryGuarantee = value.Value
		case "hub.config.Authenticator.admin_users":
			var users []string
			err := json.Unmarshal([]byte(value.Value), &users)
			if err != nil {
				return JupyterConfigurableValues{}, err
			}
			configurableValues.AdminUsers = users
		}
	}

	return configurableValues, err
}
