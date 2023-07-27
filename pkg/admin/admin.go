package admin

import (
	"github.com/nais/knorten/pkg/database"
)

type Client struct {
	repo *database.Repo
}

func New(repo *database.Repo) *Client {
	return &Client{
		repo: repo,
	}
}
