package google

import (
	"github.com/nais/knorten/pkg/database"
	"github.com/sirupsen/logrus"
)

const (
	secretRoleName = "roles/owner"
)

type Google struct {
	dryRun          bool
	log             *logrus.Entry
	repo            *database.Repo
	project         string
	region          string
	vmNetworkConfig string
}

func New(repo *database.Repo, gcpProject, gcpRegion, vmNetworkConfig string, dryRun bool, log *logrus.Entry) *Google {
	return &Google{
		log:             log,
		repo:            repo,
		project:         gcpProject,
		region:          gcpRegion,
		vmNetworkConfig: vmNetworkConfig,
		dryRun:          dryRun,
	}
}
