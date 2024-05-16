package github

import (
	"context"
	"fmt"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v62/github"
	"github.com/gregjones/httpcache"
	"github.com/navikt/knorten/pkg/interceptor"
	"net/http"
)

const (
	BaseURL             = "https://api.github.com"
	DefaultOrganization = "navikt"
)

type Lister interface {
	Repositories(ctx context.Context) ([]Repository, error)
}

type Repository struct {
	Name     string
	FullName string
	Branches []Branch
}

type Branch struct {
	Name string
}

type Client struct {
	Org string

	ghc *github.Client
}

func (c *Client) Repositories(ctx context.Context) ([]Repository, error) {
	var repositories []Repository

	opts := &github.ListOptions{}

	for {
		repos, resp, err := c.ghc.Repositories.ListByOrg(ctx, c.Org, &github.RepositoryListByOrgOptions{
			Type: "sources",
			ListOptions: github.ListOptions{
				PerPage: 100,
				Page:    opts.Page,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("listing repositories: %w", err)
		}

		for _, r := range repos {
			repositories = append(repositories, Repository{
				Name:     r.GetName(),
				FullName: r.GetFullName(),
			})
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return repositories, nil
}

func (c *Client) BranchesForRepository(ctx context.Context, r Repository) ([]Branch, error) {
	var allBranches []Branch

	opts := &github.ListOptions{}

	for {
		branches, resp, err := c.ghc.Repositories.ListBranches(ctx, c.Org, r.Name, &github.BranchListOptions{
			ListOptions: github.ListOptions{
				PerPage: 100,
				Page:    opts.Page,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("listing branches: %w", err)
		}

		for _, b := range branches {
			allBranches = append(allBranches, Branch{
				Name: b.GetName(),
			})
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return allBranches, nil

}

func New(org string, ghc *github.Client) *Client {
	return &Client{
		Org: org,
		ghc: ghc,
	}
}

func NewFromHTTPClient(org string, c *http.Client) *Client {
	return New(org, github.NewClient(c))
}

func NewHTTPClientFromGithubAppCredentials(applicationID int64, privateKey string) (*http.Client, error) {
	itr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, applicationID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("creating github client: %w", err)
	}

	// Wrap the transport in a cache
	c := httpcache.NewMemoryCacheTransport()

	chain := interceptor.InterceptorChain(c, func(_ http.RoundTripper) interceptor.InterceptorRT {
		return func(req *http.Request) (*http.Response, error) {
			return itr.RoundTrip(req)
		}
	})

	return &http.Client{
		Transport: chain,
	}, nil
}
