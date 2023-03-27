package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

func (g *Google) CreateCloudSQLInstance(ctx context.Context, dbInstance string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	sqlInstances, err := g.listSQLInstances()
	if err != nil {
		return err
	}

	if contains(sqlInstances, dbInstance) {
		g.log.Infof("create sql instance: sql instance %v already exists", dbInstance)
		return nil
	}

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
		"--memory=8GiB",
		"--require-ssl")

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

func (g *Google) DeleteCloudSQLInstance(ctx context.Context, dbInstance string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	sqlInstances, err := g.listSQLInstances()
	if err != nil {
		return err
	}

	if !contains(sqlInstances, dbInstance) {
		g.log.Infof("delete sql instance: sql instance %v does not exist", dbInstance)
		return nil
	}

	deleteCmd := exec.CommandContext(
		ctx,
		"gcloud",
		"sql",
		"instances",
		"delete",
		dbInstance)

	buf := &bytes.Buffer{}
	deleteCmd.Stdout = buf
	deleteCmd.Stderr = os.Stderr
	if err := deleteCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Error("delete db instance")
		return err
	}

	return nil
}

func (g *Google) CreateCloudSQLDatabase(ctx context.Context, dbName, dbInstance string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	sqlDBs, err := g.listSQLDatabases(dbInstance)
	if err != nil {
		return err
	}

	if contains(sqlDBs, dbName) {
		g.log.Infof("create sql database: database %v already exists in db instance %v", dbName, dbInstance)
		return nil
	}

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

func (g *Google) CreateOrUpdateCloudSQLUser(ctx context.Context, user, password, dbInstance string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	sqlUsers, err := g.listSQLUsers(dbInstance)
	if err != nil {
		return err
	}

	if contains(sqlUsers, user) {
		g.log.Infof("create sql user: updating password for user %v in db instance %v", user, dbInstance)
		return g.updateSQLUser(ctx, user, password, dbInstance)
	}

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

func (g *Google) SetSQLClientIAMBinding(ctx context.Context, teamID string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"projects",
		"add-iam-policy-binding",
		g.project,
		"--member",
		fmt.Sprintf("serviceAccount:%v@%v.iam.gserviceaccount.com", teamID, g.project),
		"--role=roles/cloudsql.client",
		"--condition=None")

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

func (g *Google) RemoveSQLClientIAMBinding(ctx context.Context, teamID string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"projects",
		"remove-iam-policy-binding",
		g.project,
		"--member",
		fmt.Sprintf("serviceAccount:%v@%v.iam.gserviceaccount.com", teamID, g.project),
		"--role=roles/cloudsql.client",
		"--condition=None")

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Error("remove sql client iam binding")
		return err
	}

	return nil
}

func (g *Google) listSQLInstances() ([]map[string]any, error) {
	listCmd := exec.Command(
		"gcloud",
		"sql",
		"instances",
		"list",
		"--format=json",
		fmt.Sprintf("--project=%v", g.project))

	buf := &bytes.Buffer{}
	listCmd.Stdout = buf
	listCmd.Stderr = os.Stderr
	if err := listCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Error("list db instances")
		return nil, err
	}

	var sqlInstances []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &sqlInstances); err != nil {
		return nil, err
	}

	return sqlInstances, nil
}

func (g *Google) listSQLDatabases(dbInstance string) ([]map[string]any, error) {
	listCmd := exec.Command(
		"gcloud",
		"sql",
		"databases",
		"list",
		"--format=json",
		fmt.Sprintf("--instance=%v", dbInstance),
		fmt.Sprintf("--project=%v", g.project))

	buf := &bytes.Buffer{}
	listCmd.Stdout = buf
	listCmd.Stderr = os.Stderr
	if err := listCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Error("list sql databases instances")
		return nil, err
	}

	var sqlDBs []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &sqlDBs); err != nil {
		return nil, err
	}

	return sqlDBs, nil
}

func (g *Google) listSQLUsers(dbInstance string) ([]map[string]any, error) {
	listCmd := exec.Command(
		"gcloud",
		"sql",
		"users",
		"list",
		"--format=json",
		fmt.Sprintf("--instance=%v", dbInstance),
		fmt.Sprintf("--project=%v", g.project))

	buf := &bytes.Buffer{}
	listCmd.Stdout = buf
	listCmd.Stderr = os.Stderr
	if err := listCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Error("list db instances")
		return nil, err
	}

	var sqlUsers []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &sqlUsers); err != nil {
		return nil, err
	}
	return sqlUsers, nil
}

func (g *Google) updateSQLUser(ctx context.Context, user, password, dbInstance string) error {
	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"sql",
		"users",
		"set-password",
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

func contains(list []map[string]any, itemName string) bool {
	for _, i := range list {
		if i["name"] == itemName {
			return true
		}
	}

	return false
}
