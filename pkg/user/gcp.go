package user

import (
	"bytes"
	"context"
	"errors"
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

	computeClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return err
	}
	defer computeClient.Close()

	computeMetadataItems := []*computepb.Items{}
	for k, v := range c.computeDefaultConfig.metadata {
		tempKey := k
		tempValue := v
		computeMetadataItems = append(computeMetadataItems, &computepb.Items{
			Key:   &tempKey,
			Value: &tempValue,
		})
	}

	req := &computepb.InsertInstanceRequest{
		Project: c.gcpProject,
		Zone:    c.gcpZone,
		InstanceResource: &computepb.Instance{
			Name:        &instanceName,
			MachineType: &c.computeDefaultConfig.machineType,
			Metadata: &computepb.Metadata{
				Items: computeMetadataItems,
			},
			NetworkInterfaces: []*computepb.NetworkInterface{
				{
					Network:    &c.computeDefaultConfig.vpcName,
					Subnetwork: &c.computeDefaultConfig.subnet,
				},
			},
			Labels: map[string]string{
				"goog-ops-agent-policy": "v2-x86-template-1-2-0",
				"created-by":            "knorten",
				"user":                  normalizeEmailToName(email),
			},
			Tags: &computepb.Tags{
				Items: []string{"knadavm"},
			},
			ServiceAccounts: []*computepb.ServiceAccount{
				{
					Email: &c.computeDefaultConfig.serviceAccount,
				},
			},
			Disks: []*computepb.AttachedDisk{
				{
					Boot:       &c.computeDefaultConfig.isBootDisk,
					DiskSizeGb: &c.computeDefaultConfig.diskSize,
					AutoDelete: &c.computeDefaultConfig.autoDeleteDisk,
					Type:       &c.computeDefaultConfig.diskType,
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						DiskName:    &instanceName,
						SourceImage: &c.computeDefaultConfig.sourceImage,
					},
				},
			},
		},
	}

	op, err := computeClient.Insert(ctx, req)
	if err != nil {
		return err
	}

	return op.Wait(ctx)
}

func (c Client) resizeComputeInstanceDiskGCP(ctx context.Context, instanceName string, diskSize int64) error {
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

	if err := c.stopComputeInstance(ctx, instanceName); err != nil {
		return err
	}

	bootDiskName, err := c.getComputeInstanceBootDiskNameGCP(ctx, instanceName)
	if err != nil {
		return err
	}

	computeClient, err := compute.NewDisksRESTClient(ctx)
	if err != nil {
		return err
	}
	defer computeClient.Close()

	req := &computepb.ResizeDiskRequest{
		Project: c.gcpProject,
		Zone:    c.gcpZone,
		Disk:    bootDiskName,
		DisksResizeRequestResource: &computepb.DisksResizeRequest{
			SizeGb: &diskSize,
		},
	}
	op, err := computeClient.Resize(ctx, req)
	if err != nil {
		return err
	}

	err = op.Wait(ctx)
	if err != nil {
		return err
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

	computeClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return err
	}
	defer computeClient.Close()

	req := &computepb.StopInstanceRequest{
		Project:  c.gcpProject,
		Zone:     c.gcpZone,
		Instance: instanceName,
	}
	op, err := computeClient.Stop(ctx, req)
	if err != nil {
		return err
	}

	err = op.Wait(ctx)
	if err != nil {
		return err
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

	computeClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return err
	}
	defer computeClient.Close()

	req := &computepb.StartInstanceRequest{
		Project:  c.gcpProject,
		Zone:     c.gcpZone,
		Instance: instanceName,
	}
	op, err := computeClient.Start(ctx, req)
	if err != nil {
		return err
	}

	return op.Wait(ctx)
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

	computeClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return err
	}
	defer computeClient.Close()

	op, err := computeClient.Delete(ctx, &computepb.DeleteInstanceRequest{
		Project:  c.gcpProject,
		Zone:     c.gcpZone,
		Instance: instanceName,
	})
	if err != nil {
		return err
	}

	return op.Wait(ctx)
}

func (c Client) computeInstanceExistsInGCP(ctx context.Context, instanceName string) (bool, error) {
	computeClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return false, err
	}
	defer computeClient.Close()

	instances := computeClient.List(ctx, &computepb.ListInstancesRequest{
		Project: c.gcpProject,
		Zone:    c.gcpZone,
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
	computeClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return "", nil
	}
	defer computeClient.Close()

	req := &computepb.GetInstanceRequest{
		Project:  c.gcpProject,
		Zone:     c.gcpZone,
		Instance: instanceName,
	}
	resp, err := computeClient.Get(ctx, req)
	if err != nil {
		return "", nil
	}

	return getBootDiskName(resp.Disks)
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
	role := "roles/owner"
	computeClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return err
	}
	defer computeClient.Close()

	req := &computepb.GetIamPolicyInstanceRequest{
		Project:  c.gcpProject,
		Zone:     c.gcpZone,
		Resource: instanceName,
	}
	policy, err := computeClient.GetIamPolicy(ctx, req)
	if err != nil {
		return err
	}

	policy = addPolicyBindingMember(policy, role, user)
	setReq := &computepb.SetIamPolicyInstanceRequest{
		Project:  c.gcpProject,
		Zone:     c.gcpZone,
		Resource: instanceName,
		ZoneSetPolicyRequestResource: &computepb.ZoneSetPolicyRequest{
			Policy: policy,
		},
	}

	_, err = computeClient.SetIamPolicy(ctx, setReq)
	if err != nil {
		return err
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

func getBootDiskName(instanceDisks []*computepb.AttachedDisk) (string, error) {
	for _, d := range instanceDisks {
		if d.Boot != nil && *d.Boot {
			return diskNameFromDiskSource(d.Source)
		}
	}
	return "", errors.New("getting boot disk name: compute instance has no boot disk")
}

func diskNameFromDiskSource(diskSource *string) (string, error) {
	if diskSource == nil {
		return "", errors.New("diskSource for compute instance is nil")
	}

	sourceParts := strings.Split(*diskSource, "/")
	return sourceParts[len(sourceParts)-1], nil
}

func addPolicyBindingMember(policy *computepb.Policy, role, email string) *computepb.Policy {
	for _, binding := range policy.Bindings {
		if binding.Role != nil && *binding.Role == role {
			binding.Members = append(binding.Members, fmt.Sprintf("user:%v", email))
			return policy
		}
	}

	policy.Bindings = append(policy.Bindings, &computepb.Binding{
		Members: []string{fmt.Sprintf("user:%v", email)},
		Role:    &role,
	})

	return policy
}
