package google

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

func (g *Google) CreateCloudSQLInstance(ctx context.Context, dbInstance string) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 25*time.Minute)
	cmd := exec.CommandContext(
		ctxWithTimeout,
		"gcloud",
		"sql",
		"instances",
		"create",
		dbInstance,
		fmt.Sprintf("--project=%v", g.project),
		fmt.Sprintf("--region=%v", g.region),
		"--database-version=POSTGRES_14",
		"--cpu=2",
		"--memory=8GiB")

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		cancel()
		g.log.WithError(err).Error("create db instance")
		return err
	}

	cancel()
	return nil
}

func (g *Google) CreateCloudSQLDatabase(ctx context.Context, dbName, dbInstance string) error {
	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"sql",
		"databases",
		"create",
		dbName,
		fmt.Sprintf("--instance=%v", dbInstance),
		fmt.Sprintf("--project=%v", g.project))

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Error("create db")
		return err
	}

	return nil
}

func (g *Google) CreateCloudSQLUser(ctx context.Context, user, password, dbInstance string) error {
	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"sql",
		"users",
		"create",
		user,
		fmt.Sprintf("--password=%v", password),
		fmt.Sprintf("--instance=%v", dbInstance),
		fmt.Sprintf("--project=%v", g.project))

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Error("create db user")
		return err
	}

	return nil
}

func (g *Google) CreateSQLClientIAMBinding(ctx context.Context, team string) error {
	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"projects",
		"add-iam-policy-binding",
		g.project,
		"--member",
		fmt.Sprintf("serviceAccount:%v@%v.iam.gserviceaccount.com", team, g.project),
		"--role",
		"roles/cloudsql.client")

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Error("create sql client iam binding")
		return err
	}

	return nil
}
