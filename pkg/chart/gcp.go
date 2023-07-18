package chart

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func (a airflowClient) deleteCloudSQLInstance(ctx context.Context, teamID string) error {
	if a.dryRun {
		a.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	instanceName := createAirflowcloudSQLInstanceName(teamID)

	exisits, err := a.cloudSQLInstanceExisits(ctx, instanceName)
	if err != nil {
		return err
	}
	if !exisits {
		a.log.Infof("delete sql instance: sql instance %v does not exist", instanceName)
		return nil
	}

	deleteCmd := exec.CommandContext(
		ctx,
		"gcloud",
		"sql",
		"instances",
		"delete",
		instanceName,
		"--project", a.gcpProject)

	buf := &bytes.Buffer{}
	deleteCmd.Stdout = buf
	deleteCmd.Stderr = os.Stderr
	if err := deleteCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		a.log.WithError(err).Error("delete db instance")
		return err
	}

	return nil
}

func (a airflowClient) cloudSQLInstanceExisits(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"sql",
		"instances",
		"list",
		"--format=get(name)",
		"--project", a.gcpProject,
		"--filter", name)

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return false, err
	}

	var instances []string
	if err := json.Unmarshal(buf.Bytes(), &instances); err != nil {
		return false, err
	}

	return len(instances) > 0, nil
}

func createAirflowcloudSQLInstanceName(teamID string) string {
	return "airflow-" + teamID
}

func (a airflowClient) removeSQLClientIAMBinding(ctx context.Context, teamID string) error {
	if a.dryRun {
		a.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"projects",
		"remove-iam-policy-binding",
		a.gcpProject,
		"--member",
		fmt.Sprintf("serviceAccount:%v@%v.iam.gserviceaccount.com", teamID, a.gcpProject),
		"--role=roles/cloudsql.client",
		"--condition=None")

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		a.log.WithError(err).Error("remove sql client iam binding")
		return err
	}

	return nil
}
