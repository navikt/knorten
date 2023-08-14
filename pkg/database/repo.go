package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/postgres"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"

	// Pin version of sqlc cli
	_ "github.com/kyleconroy/sqlc"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Repository interface {
	EventSetStatus(context.Context, uuid.UUID, gensql.EventStatus) error
	EventSetPendingStatus(context.Context, uuid.UUID) error
	DispatcherEventsGet(context.Context) ([]gensql.Event, error)
	EventLogCreate(context.Context, uuid.UUID, string, gensql.LogType) error
}

type Repo struct {
	querier     Querier
	db          *sql.DB
	cryptClient *crypto.EncrypterDecrypter
	log         *logrus.Entry
}

type Querier interface {
	gensql.Querier
	WithTx(tx *sql.Tx) *gensql.Queries
}

func New(dbConnDSN, cryptoKey string, log *logrus.Entry) (*Repo, error) {
	db, err := sql.Open("postgres", dbConnDSN)
	if err != nil {
		return nil, fmt.Errorf("open sql connection: %w", err)
	}

	err = gooseMigrationWithRetries(log, db)
	if err != nil {
		return nil, fmt.Errorf("goose up: %w", err)
	}

	return &Repo{
		querier:     gensql.New(db),
		db:          db,
		cryptClient: crypto.New(cryptoKey),
		log:         log,
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

func (r *Repo) DecryptValue(encValue string) (string, error) {
	return r.cryptClient.DecryptValue(encValue)
}

func (r *Repo) EncryptValue(encValue string) (string, error) {
	return r.cryptClient.EncryptValue(encValue)
}
