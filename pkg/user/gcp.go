package user

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/navikt/knorten/pkg/gcp"
)

var gcpIAMPolicyBindingsRoles = []string{
	"roles/compute.viewer",
	"roles/iap.tunnelResourceAccessor",
	"roles/monitoring.viewer",
}

const opsServiceAccountEmail = "knada-vm-ops-agent@knada-gcp.iam.gserviceaccount.com"

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
		fmt.Sprintf("--labels=goog-ops-agent-policy=v2-x86-template-1-2-0,created-by=knorten,user=%v", normalizeEmailToName(email)),
		"--tags=knadavm",
		"--metadata=block-project-ssh-keys=TRUE,enable-osconfig=TRUE",
		"--service-account", opsServiceAccountEmail,
		"--no-scopes",
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

func (c Client) resizeComputeInstanceDiskGCP(ctx context.Context, instanceName string, diskSize int32) error {
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
		"disks",
		"resize",
		instanceName,
		fmt.Sprintf("--project=%v", c.gcpProject),
		fmt.Sprintf("--zone=%v", c.gcpZone),
		fmt.Sprintf("--size=%vGB", diskSize),
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

func (c Client) createIAMPolicyBindingsInGCP(ctx context.Context, instanceName, email string) error {
	if c.dryRun {
		return nil
	}

	if err := c.addComputeInstanceOwnerBindingInGCP(ctx, instanceName, email); err != nil {
		return err
	}

	if err := c.addOpsServiceAccountUserBinding(ctx, email); err != nil {
		return err
	}

	for _, role := range gcpIAMPolicyBindingsRoles {
		if err := c.addProjectIAMPolicyBindingInGCP(ctx, instanceName, email, role); err != nil {
			return err
		}
	}

	return nil
}

func (c Client) deleteIAMPolicyBindingsFromGCP(ctx context.Context, instanceName, email string) error {
	if c.dryRun {
		return nil
	}

	if err := c.removeOpsServiceAccountUserBinding(ctx, email); err != nil {
		return err
	}

	for _, role := range gcpIAMPolicyBindingsRoles {
		if err := c.removeProjectIAMPolicyBindingFromGCP(ctx, instanceName, email, role); err != nil {
			return err
		}
	}

	return nil
}

func (c Client) addComputeInstanceOwnerBindingInGCP(ctx context.Context, instanceName, user string) error {
	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"compute",
		"instances",
		"add-iam-policy-binding",
		instanceName,
		"--zone", c.gcpZone,
		"--project", c.gcpProject,
		"--role", "roles/owner",
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

func (c Client) addOpsServiceAccountUserBinding(ctx context.Context, email string) error {
	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"iam",
		"service-accounts",
		"add-iam-policy-binding",
		opsServiceAccountEmail,
		"--project", c.gcpProject,
		"--role", "roles/iam.serviceAccountUser",
		fmt.Sprintf("--member=user:%v", email),
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

func (c Client) removeOpsServiceAccountUserBinding(ctx context.Context, email string) error {
	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"iam",
		"service-accounts",
		"remove-iam-policy-binding",
		opsServiceAccountEmail,
		"--project", c.gcpProject,
		"--role", "roles/iam.serviceAccountUser",
		fmt.Sprintf("--member=user:%v", email),
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

func (c Client) addProjectIAMPolicyBindingInGCP(ctx context.Context, instanceName, user, role string) error {
	if c.dryRun {
		return nil
	}

	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"projects",
		"add-iam-policy-binding",
		c.gcpProject,
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

func (c Client) removeProjectIAMPolicyBindingFromGCP(ctx context.Context, instanceName, user, role string) error {
	if c.dryRun {
		return nil
	}

	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"projects",
		"remove-iam-policy-binding",
		c.gcpProject,
		"--role", role,
		fmt.Sprintf("--member=user:%v", user),
	)

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stdErr.String(), "not found") {
			return nil
		}

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
