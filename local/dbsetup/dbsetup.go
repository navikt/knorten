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

	airflowContainer := func(name string) string {
		return fmt.Sprintf(`[{"name": "%v", "image": "registry.k8s.io/git-sync/git-sync:v3.6.3","args": ["", "", "/dags", "60"], "volumeMounts":[{"mountPath":"/dags","name":"dags"}]}]`, name)
	}

	fmt.Println("Time to insert dummy data for local development")
	rows := [][]interface{}{
		{"airflow", "config.core.dags_folder", `"/dags"`},
		{"airflow", "createUserJob.serviceAccount.create", "false"},
		{"airflow", "postgresql.enabled", "false"},
		{"airflow", "scheduler.extraContainers", airflowContainer("git-nada")},
		{"airflow", "scheduler.extraInitContainers", airflowContainer("git-nada-clone")},
		{"airflow", "webserver.extraContainers", airflowContainer("git-nada")},
		{"airflow", "webserver.serviceAccount.create", "false"},
		{"airflow", "webserverSecretKeySecretName", "airflow-webserver"},
		{"airflow", "workers.extraInitContainers", airflowContainer("git-nada")},
		{"airflow", "workers.serviceAccount.create", "false"},
		{"jupyterhub", "singleuser.profileList", "[]"},
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
