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
	computeDefaultConfig computeInstanceConfig
	opsAgentSAResource   string
	dryRun               bool
}

func NewClient(repo *database.Repo, gcpProject, gcpRegion, gcpZone string, dryRun bool) *Client {
	return &Client{
		repo:                 repo,
		gcpProject:           gcpProject,
		gcpRegion:            gcpRegion,
		gcpZone:              gcpZone,
		dryRun:               dryRun,
		opsAgentSAResource:   fmt.Sprintf("projects/%v/serviceAccounts/knada-vm-ops-agent@%v.iam.gserviceaccount.com", gcpProject, gcpProject),
		computeDefaultConfig: newComputeDefaultConfig(gcpProject, gcpRegion, gcpZone),
	}
}
