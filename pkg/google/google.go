package google

import (
	"github.com/nais/knorten/pkg/database"
	"github.com/sirupsen/logrus"
)

type Google struct {
	dryRun  bool
	log     *logrus.Entry
	repo    *database.Repo
	project string
	region  string
}

func New(repo *database.Repo, gcpProject, gcpRegion string, dryRun bool, log *logrus.Entry) *Google {
	return &Google{
		log:     log,
		repo:    repo,
		project: gcpProject,
		region:  gcpRegion,
		dryRun:  dryRun,
	}
}
