package user

import (
	"fmt"

	"github.com/navikt/knorten/pkg/database"
)

type Client struct {
	repo                 *database.Repo
	gcpProject           string
	gcpRegion            string
	gcpZone              string
	computeDefaultConfig computeInstanceDefaultConfig
	dryRun               bool
}

type computeInstanceDefaultConfig struct {
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

func NewClient(repo *database.Repo, gcpProject, gcpRegion, gcpZone string, dryRun bool) *Client {
	return &Client{
		repo:                 repo,
		gcpProject:           gcpProject,
		gcpRegion:            gcpRegion,
		gcpZone:              gcpZone,
		dryRun:               dryRun,
		computeDefaultConfig: newComputeDefaultConfig(gcpProject, gcpRegion, gcpZone),
	}
}

func newComputeDefaultConfig(gcpProject, gcpRegion, gcpZone string) computeInstanceDefaultConfig {
	return computeInstanceDefaultConfig{
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
