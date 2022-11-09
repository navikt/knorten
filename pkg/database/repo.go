package database

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
	"reflect"
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

func (r *Repo) TeamConfigurableValuesGet(ctx context.Context, chartType gensql.ChartType, team string, obj any) error {
	teamValues, err := r.querier.TeamValuesGet(ctx, gensql.TeamValuesGetParams{
		ChartType: chartType,
		Team:      team,
	})

	for _, value := range teamValues {
		err := InterfaceToStruct(obj, value.Key, value.Value)
		if err != nil {
			return err
		}
	}

	return err
}

func InterfaceToStruct(obj any, tag string, value string) error {
	fieldName, err := findFieldNameByTag(tag, obj)
	if err != nil {
		return err
	}

	structValue := reflect.ValueOf(obj).Elem()
	structFieldValue := structValue.FieldByName(fieldName)

	if !structFieldValue.IsValid() {
		return fmt.Errorf("no such field: %s in obj", fieldName)
	}

	if !structFieldValue.CanSet() {
		return fmt.Errorf("cannot set %s field value", fieldName)
	}

	kind := structFieldValue.Kind()
	switch kind {
	case reflect.String:
		structFieldValue.Set(reflect.ValueOf(value))
	case reflect.Slice:
		var users []string
		err := json.Unmarshal([]byte(value), &users)
		if err != nil {
			return err
		}
		structFieldValue.Set(reflect.ValueOf(users))
	default:
		return fmt.Errorf("unknown kind('%v')", kind)
	}

	return nil
}

func findFieldNameByTag(tag string, obj any) (string, error) {
	structValue := reflect.ValueOf(obj).Elem()
	fields := reflect.VisibleFields(structValue.Type())

	for _, field := range fields {
		fieldTag := field.Tag.Get("helm")
		if tag == fieldTag {
			return field.Name, nil
		}
	}

	return "", fmt.Errorf("can't find 'helm' tag with the value: '%v'", tag)
}
