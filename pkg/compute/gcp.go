package compute

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
)

const (
	computeZone = "europe-west1-b"
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
			fmt.Sprintf("--zone=%v", computeZone),
			fmt.Sprintf("--machine-type=%v", "n2-standard-2"),
			fmt.Sprintf("--network-interface=%v", "network=knada-vpc,subnet=knada,no-address"),
			fmt.Sprintf("--labels=created-by=knorten,user=%v", email),
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
		fmt.Sprintf("--project=%v", c.gcpProject),
		fmt.Sprintf("--filter=name=%v", name))

	buf := &bytes.Buffer{}
	listCmd.Stdout = buf
	listCmd.Stderr = os.Stderr
	if err := listCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return false, err
	}

	var instances []string
	if err := json.Unmarshal(buf.Bytes(), &instances); err != nil {
		return false, err
	}

	return len(instances) > 0, nil
}

func (c Client) addGCPOwnerBinding(ctx context.Context, instance, user string) error {
	if c.dryRun {
		return nil
	}

	addCmd := exec.CommandContext(
		ctx,
		"gcloud",
		"compute",
		"instances",
		"add-iam-policy-binding",
		instance,
		fmt.Sprintf("--role=%v", "roles/owner"),
		fmt.Sprintf("--member=user:%v", user),
		fmt.Sprintf("--zone=%v", computeZone),
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

func (c Client) deleteComputeInstanceFromGCP(ctx context.Context, instance string) error {
	if c.dryRun {
		return nil
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"compute",
		"instances",
		"delete",
		instance,
		fmt.Sprintf("--zone=%v", computeZone),
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
