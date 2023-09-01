package user

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/nais/knorten/pkg/gcp"
)

func (c Client) createComputeInstanceInGCP(ctx context.Context, instanceName, email string) error {
	if c.dryRun {
		return nil
	}

	exists, err := c.computeInstanceExistsInGCP(ctx, instanceName)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"compute",
		"instances",
		"create",
		instanceName,
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

	if err := c.addGCPOwnerBinding(ctx, instanceName, email); err != nil {
		return err
	}

	if err := c.addGCPKnadaVMUserBinding(ctx, instanceName, email); err != nil {
		return err
	}

	return nil
}

func (c Client) computeInstanceExistsInGCP(ctx context.Context, instanceName string) (bool, error) {
	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"compute",
		"instances",
		"list",
		"--format=get(name)",
		"--project", c.gcpProject,
		fmt.Sprintf("--filter=name:%v", instanceName))

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
	return c.addGCPIAMPolicyBinding(ctx, instanceName, user, "roles/owner")
}

func (c Client) addGCPKnadaVMUserBinding(ctx context.Context, instanceName, user string) error {
	return c.addGCPIAMPolicyBinding(ctx, instanceName, user, "projects/knada-gcp/roles/knadvmauser")
}

func (c Client) addGCPIAMPolicyBinding(ctx context.Context, instanceName, user, role string) error {
	if c.dryRun {
		return nil
	}

	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"compute",
		"instances",
		"add-iam-policy-binding",
		instanceName,
		"--zone", c.gcpZone,
		"--project", c.gcpProject,
		"--role", role,
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

func (c Client) deleteIAMPolicyBinding(ctx context.Context, instanceName, user string) error {
	if c.dryRun {
		return nil
	}

	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"compute",
		"instances",
		"remove-iam-policy-binding",
		instanceName,
		"--zone", c.gcpZone,
		"--project", c.gcpProject,
		"--role", "projects/knada-gcp/roles/knadvmauser",
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

	exists, err := c.computeInstanceExistsInGCP(ctx, instanceName)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	cmd := exec.CommandContext(ctx,
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

func (c Client) createUserGSMInGCP(ctx context.Context, name, owner string) error {
	if c.dryRun {
		return nil
	}

	secret, err := gcp.CreateSecret(ctx, c.gcpProject, c.gcpRegion, name, map[string]string{"owner": name})
	if err != nil {
		return err
	}

	if err := gcp.SetUsersSecretOwnerBinding(ctx, []string{owner}, secret.Name); err != nil {
		return err
	}

	return nil
}

func (c Client) deleteUserGSMFromGCP(ctx context.Context, name string) error {
	if c.dryRun {
		return nil
	}

	if err := gcp.DeleteSecret(ctx, c.gcpProject, name); err != nil {
		return err
	}

	return nil
}
