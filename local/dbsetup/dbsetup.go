package dbsetup

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func SetupDB(ctx context.Context, dbURL, dbname string) error {
	db, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	fmt.Println("Successfully connected!")

	if err := db.QueryRow(ctx, "SELECT FROM pg_catalog.pg_database WHERE datname = $1", dbname).Scan(); err != nil {
		if err == pgx.ErrNoRows {
			fmt.Printf("Creating database %v\n", dbname)
			_, err := db.Exec(ctx, fmt.Sprintf("CREATE DATABASE %v", dbname))
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	err = db.Close(ctx)
	if err != nil {
		return err
	}

	db, err = pgx.Connect(ctx, dbURL+"/"+dbname)
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	if err := db.QueryRow(ctx, "SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename  = $1", "goose_db_version").Scan(); err != nil {
		if err == pgx.ErrNoRows {
			fmt.Println("You need to run `make goose cmd=up`")
			os.Exit(1)
		} else {
			return err
		}
	}

	var oid uint32
	err = db.QueryRow(context.Background(), "select oid from pg_type where typname=$1;", "chart_type").Scan(&oid)
	if err != nil {
		return err
	}

	db.TypeMap().RegisterType(&pgtype.Type{Name: "chart_type", OID: oid, Codec: &pgtype.EnumCodec{}})

	fmt.Println("Time to insert dummy data for local development")
	rows := [][]interface{}{
		{"airflow", "scheduler.extraContainers", `[{"name":"dummy","image":"navikt/dummy:aaa15ba","args":["","","/dags","60"]}]`},
		{"airflow", "scheduler.extraInitContainers", `[{"name":"dummy","image":"navikt/dummy:aaa15ba","args":["","","/dags","60"]}]`},
		{"airflow", "webserver.extraContainers", `[{"name":"dummy","image":"navikt/dummy:aaa15ba","args":["","","/dags","60"]}]`},
		{"airflow", "workers.extraInitContainers", `[{"name":"dummy","image":"navikt/dummy:aaa15ba","args":["","","/dags","60"]}]`},
	}
	_, err = db.CopyFrom(ctx,
		pgx.Identifier{"chart_global_values"},
		[]string{"chart_type", "key", "value"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return err
	}

	return nil
}
