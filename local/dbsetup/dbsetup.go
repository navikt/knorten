package dbsetup

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ory/dockertest/v3"
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
		{"airflow", "images.airflow.repository", "europe-north1-docker.pkg.dev/knada-gcp/knada-north/airflow"},
		{"airflow", "images.airflow.tag", "2024-02-16-d06f032"},
		{"airflow", "env", `[{"name":"CLONE_REPO_IMAGE","value":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/git-sync:2024-01-19-0d0d790"},{"name":"KNADA_AIRFLOW_OPERATOR_IMAGE","value":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/dataverk-airflow:2024-01-12-09bd685"},{"name":"DATAVERK_IMAGE_PYTHON_38","value":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/dataverk-airflow-python-3.8:2024-02-16-d06f032"},{"name":"DATAVERK_IMAGE_PYTHON_39","value":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/dataverk-airflow-python-3.9:2024-02-09-15e79cd"},{"name":"DATAVERK_IMAGE_PYTHON_310","value":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/dataverk-airflow-python-3.10:2024-02-09-15e79cd"},{"name":"DATAVERK_IMAGE_PYTHON_311","value":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/dataverk-airflow-python-3.11:2024-02-09-15e79cd"},{"name":"DATAVERK_IMAGE_PYTHON_312","value":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/dataverk-airflow-python-3.12:2024-02-16-d06f032"}]`},
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

func SetupDBForTests() (string, error) {
	dbString := "user=postgres dbname=knorten sslmode=disable password=postgres host=db port=5432"

	if os.Getenv("CLOUDBUILD") != "true" {
		dockerHost := os.Getenv("HOME") + "/.colima/docker.sock"
		_, err := os.Stat(dockerHost)
		if err != nil {
			// uses a sensible default on windows (tcp/http) and linux/osx (socket)
			dockerHost = ""
		} else {
			dockerHost = "unix://" + dockerHost
		}

		pool, err := dockertest.NewPool(dockerHost)
		if err != nil {
			log.Fatalf("Could not connect to docker: %s", err)
		}

		// pulls an image, creates a container based on it and runs it
		resource, err := pool.Run("postgres", "14", []string{"POSTGRES_PASSWORD=postgres", "POSTGRES_DB=knorten"})
		if err != nil {
			log.Fatalf("Could not start resource: %s", err)
		}

		// setting resource timeout as postgres container is not terminated automatically
		if err = resource.Expire(120); err != nil {
			log.Fatalf("failed creating postgres expire: %v", err)
		}

		dbPort := resource.GetPort("5432/tcp")
		dbString = fmt.Sprintf("user=postgres dbname=knorten sslmode=disable password=postgres host=localhost port=%v", dbPort)
	}

	if err := waitForDB(dbString); err != nil {
		log.Fatal(err)
	}

	return dbString, nil
}

func waitForDB(dbString string) error {
	sleepDuration := 1 * time.Second
	numRetries := 60
	for i := 0; i < numRetries; i++ {
		time.Sleep(sleepDuration)
		db, err := sql.Open("postgres", dbString)
		if err != nil {
			return err
		}

		if err := db.Ping(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("unable to connect to db in %v seconds", int(sleepDuration)*numRetries/1000000000)
}
