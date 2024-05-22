package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
)

type computeInstanceConfig struct {
	vpcName        string
	subnet         string
	serviceAccount string
	machineType    string
	isBootDisk     bool
	autoDeleteDisk bool
	diskType       string
	diskSize       int64
	sourceImage    string
	metadata       map[string]string
}

func newComputeDefaultConfig(gcpProject, gcpRegion, gcpZone string) computeInstanceConfig {
	return computeInstanceConfig{
		vpcName:        "projects/knada-dev/global/networks/knada-vpc",
		subnet:         fmt.Sprintf("projects/knada-dev/regions/%v/subnetworks/knada", gcpRegion),
		serviceAccount: fmt.Sprintf("knada-vm-ops-agent@%v.iam.gserviceaccount.com", gcpProject),
		machineType:    fmt.Sprintf("zones/%v/machineTypes/n2-standard-2", gcpZone),
		isBootDisk:     true,
		autoDeleteDisk: true,
		diskType:       "PERSISTENT",
		diskSize:       int64(20),
		sourceImage:    "projects/debian-cloud/global/images/family/debian-11",

		metadata: map[string]string{
			"block-project-ssh-keys": "TRUE",
			"enable-osconfig":        "TRUE",
		},
	}
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
		keyCopy := k
		valueCopy := v
		computeMetadataItems = append(computeMetadataItems, &computepb.Items{
			Key:   &keyCopy,
			Value: &valueCopy,
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

func normalizeEmailToName(email string) string {
	name, _ := strings.CutSuffix(email, "@nav.no")
	return strings.ReplaceAll(name, ".", "_")
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
