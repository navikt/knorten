package compute

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func (c Client) createComputeInstanceInGCP(ctx context.Context, name, email string) error {
	if c.dryRun {
		return nil
	}

	exists, err := c.computeInstanceExistsInGCP(name)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"--quiet",
		"compute",
		"instances",
		"create",
		name,
		"--project", c.gcpProject,
		"--zone", c.gcpZone,
		"--machine-type", "n2-standard-2",
		"--network-interface", "network=knada-vpc,subnet=knada,no-address",
		fmt.Sprintf("--labels=created-by=knorten,user=%v", normalizeEmailToName(email)),
		"--metadata=block-project-ssh-keys=TRUE",
		"--no-service-account",
		"--no-scopes",
	)

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	if err := c.addGCPOwnerBinding(ctx, name, email); err != nil {
		return err
	}

	return nil
}

func (c Client) computeInstanceExistsInGCP(name string) (bool, error) {
	cmd := exec.Command(
		"gcloud",
		"--quiet",
		"compute",
		"instances",
		"list",
		"--format=get(name)",
		"--project", c.gcpProject,
		fmt.Sprintf("--filter=name=%v", name))

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return stdOut.String() != "", nil
}

func (c Client) addGCPOwnerBinding(ctx context.Context, instanceName, user string) error {
	if c.dryRun {
		return nil
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"--quiet",
		"compute",
		"instances",
		"add-iam-policy-binding",
		instanceName,
		"--zone", c.gcpZone,
		"--project", c.gcpProject,
		fmt.Sprintf("--role=%v", "roles/owner"),
		fmt.Sprintf("--member=user:%v", user),
	)

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}

func (c Client) deleteComputeInstanceFromGCP(ctx context.Context, instanceName string) error {
	if c.dryRun {
		return nil
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"--quiet",
		"compute",
		"instances",
		"delete",
		"--delete-disks=all",
		instanceName,
		"--zone", c.gcpZone,
		"--project", c.gcpProject,
	)

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}

func normalizeEmailToName(email string) string {
	name, _ := strings.CutSuffix(email, "@nav.no")
	return strings.ReplaceAll(name, ".", "_")
}
