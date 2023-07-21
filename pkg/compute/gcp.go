package compute

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
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
	if !exists {
		cmd := exec.CommandContext(
			ctx,
			"gcloud",
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

		buf := &bytes.Buffer{}
		cmd.Stdout = buf
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			io.Copy(os.Stdout, buf)
			return err
		}
	}

	if err := c.addGCPOwnerBinding(ctx, name, email); err != nil {
		return err
	}

	return nil
}

func (c Client) computeInstanceExistsInGCP(name string) (bool, error) {
	listCmd := exec.Command(
		"gcloud",
		"compute",
		"instances",
		"list",
		"--format=get(name)",
		"--project", c.gcpProject,
		fmt.Sprintf("--filter=name=%v", name))

	buf := &bytes.Buffer{}
	listCmd.Stdout = buf
	listCmd.Stderr = os.Stderr
	if err := listCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return false, err
	}

	return buf.String() != "", nil
}

func (c Client) addGCPOwnerBinding(ctx context.Context, instanceName, user string) error {
	if c.dryRun {
		return nil
	}

	addCmd := exec.CommandContext(
		ctx,
		"gcloud",
		"compute",
		"instances",
		"add-iam-policy-binding",
		instanceName,
		"--zone", c.gcpZone,
		"--project", c.gcpProject,
		fmt.Sprintf("--role=%v", "roles/owner"),
		fmt.Sprintf("--member=user:%v", user),
	)

	buf := &bytes.Buffer{}
	addCmd.Stdout = buf
	addCmd.Stderr = os.Stderr
	if err := addCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return err
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
		"compute",
		"instances",
		"delete",
		"--delete-disks=all",
		"--quiet",
		instanceName,
		"--zone", c.gcpZone,
		"--project", c.gcpProject,
	)

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return err
	}

	return nil
}

func normalizeEmailToName(email string) string {
	name, _ := strings.CutSuffix(email, "@nav.no")
	return strings.ReplaceAll(name, ".", "_")
}
