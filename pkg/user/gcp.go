package user

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/navikt/knorten/pkg/gcp"
	"google.golang.org/api/iterator"
)

var gcpIAMPolicyBindingsRoles = []string{
	"roles/compute.viewer",
	"roles/iap.tunnelResourceAccessor",
	"roles/monitoring.viewer",
}

type computeInstance struct {
	Name  string `json:"name"`
	Disks []disk `json:"disks"`
}

type disk struct {
	Source string `json:"source"`
	Boot   bool   `json:"boot"`
}

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
		"--service-account", fmt.Sprintf("knada-vm-ops-agent@%v.iam.gserviceaccount.com", c.gcpProject),
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

	if !exists {
		return nil
	}

	bootDiskName, err := c.getComputeInstanceBootDiskNameGCP(ctx, instanceName)
	if err != nil {
		return err
	}

	if err := c.stopComputeInstance(ctx, instanceName); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"compute",
		"disks",
		"resize",
		bootDiskName,
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

	return c.startComputeInstance(ctx, instanceName)
}

func (c Client) stopComputeInstance(ctx context.Context, instanceName string) error {
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
		"stop",
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

func (c Client) startComputeInstance(ctx context.Context, instanceName string) error {
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
		"start",
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
	computeClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return false, err
	}
	defer computeClient.close()

	instances := computeClient.List(ctx, &computepb.ListInstancesRequest{
		Project: "knada-dev",
		Zone:    "europe-north1-b",
	})
	for {
		instance, err := instances.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return false, err
		}

		if instance.Name != nil && *instance.Name == instanceName {
			return true, nil
		}
	}

	return false, nil
}

func (c Client) getComputeInstanceBootDiskNameGCP(ctx context.Context, instanceName string) (string, error) {
	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"compute",
		"instances",
		"describe",
		instanceName,
		"--zone", c.gcpZone,
		"--project", c.gcpProject,
		"--format=json")

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	instance := computeInstance{}
	if err := json.Unmarshal(stdOut.Bytes(), &instance); err != nil {
		return "", err
	}

	return getBootDiskName(instance)
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
		"--condition=None",
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
		fmt.Sprintf("knada-vm-ops-agent@%v.iam.gserviceaccount.com", c.gcpProject),
		"--project", c.gcpProject,
		"--role", "roles/iam.serviceAccountUser",
		"--condition=None",
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
		fmt.Sprintf("knada-vm-ops-agent@%v.iam.gserviceaccount.com", c.gcpProject),
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
		"--condition=None",
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

func getBootDiskName(instance computeInstance) (string, error) {
	for _, d := range instance.Disks {
		if d.Boot {
			return diskNameFromDiskSource(d), nil
		}
	}
	return "", fmt.Errorf("getting boot disk name: compute instance %v does not have a boot disk", instance.Name)
}

func diskNameFromDiskSource(disk disk) string {
	sourceParts := strings.Split(disk.Source, "/")
	return sourceParts[len(sourceParts)-1]
}
