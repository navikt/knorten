package database

import (
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/postgres"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"

	// Pin version of sqlc cli
	_ "github.com/kyleconroy/sqlc"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Repo struct {
	querier Querier
	encKey  string
	db      *sql.DB
	log     *logrus.Entry
}

type Querier interface {
	gensql.Querier
	WithTx(tx *sql.Tx) *gensql.Queries
}

func New(dbConnDSN string, log *logrus.Entry) (gensql.Querier, *Repo, error) {
	db, err := sql.Open("postgres", dbConnDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("open sql connection: %w", err)
	}

	err = gooseMigrationWithRetries(log, db)
	if err != nil {
		return nil, nil, fmt.Errorf("goose up: %w", err)
	}

	querier := gensql.New(db)

	return querier, &Repo{
		querier: querier,
		db:      db,
		log:     log,
	}, nil
}

func gooseMigrationWithRetries(log *logrus.Entry, db *sql.DB) error {
	goose.SetLogger(log)
	goose.SetBaseFS(embedMigrations)

	err := goose.Up(db, "migrations")
	if err != nil {
		backoffSchedule := []time.Duration{
			5 * time.Second,
			15 * time.Second,
			30 * time.Second,
			60 * time.Second,
		}

		for _, duration := range backoffSchedule {
			time.Sleep(duration)
			err = goose.Up(db, "migrations")
			if err == nil {
				return nil
			}
		}
	}

	return err
}

func (r *Repo) NewSessionStore(key string) (gin.HandlerFunc, error) {
	store, err := postgres.NewStore(r.db, []byte(key))
	if err != nil {
		return nil, err
	}

	return sessions.Sessions("session", store), nil
}
