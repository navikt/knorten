package user

import (
	"github.com/navikt/knorten/pkg/database"
)

type Client struct {
	repo       *database.Repo
	gcpProject string
	gcpRegion  string
	gcpZone    string
	dryRun     bool
}

func NewClient(repo *database.Repo, gcpProject, gcpRegion, gcpZone string, dryRun bool) *Client {
	return &Client{
		repo:       repo,
		gcpProject: gcpProject,
		gcpRegion:  gcpRegion,
		gcpZone:    gcpZone,
		dryRun:     dryRun,
	}
}
