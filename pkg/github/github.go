package github

import (
	"context"
	"fmt"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v62/github"
	"github.com/gregjones/httpcache"
	"github.com/sirupsen/logrus"
	"net/http"
	"sync"
	"time"
)

const (
	DefaultOrganization = "navikt"
)

type Lister interface {
	Repositories(ctx context.Context) ([]Repository, error)
	Branches(ctx context.Context, r Repository) ([]Branch, error)
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

func (c *Client) Branches(ctx context.Context, r Repository) ([]Branch, error) {
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

func NewHTTPClientFromGithubAppCredentials(applicationID, installationID int64, privateKey string) (*http.Client, error) {
	cacher := httpcache.NewMemoryCacheTransport()

	itr, err := ghinstallation.NewKeyFromFile(cacher, applicationID, installationID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("creating github transport: %w", err)
	}

	return &http.Client{
		Transport: itr,
	}, nil
}

type StaticLister struct {
	repositories []Repository
	branches     map[string][]Branch
}

func NewStaticLister(repos []Repository, branches map[string][]Branch) *StaticLister {
	return &StaticLister{
		repositories: repos,
		branches:     branches,
	}
}

func (s *StaticLister) Repositories(_ context.Context) ([]Repository, error) {
	return s.repositories, nil
}

func (s *StaticLister) Branches(_ context.Context, r Repository) ([]Branch, error) {
	return s.branches[r.Name], nil
}

type Service struct {
	log          *logrus.Entry
	lister       Lister
	repositories map[string]Repository
	mu           sync.RWMutex
}

func NewService(l Lister, log *logrus.Entry) *Service {
	return &Service{
		log:          log,
		lister:       l,
		repositories: make(map[string]Repository),
	}
}

func (s *Service) Refresh(ctx context.Context) (int, error) {
	repos, err := s.lister.Repositories(ctx)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, repo := range repos {
		s.repositories[repo.Name] = repo
	}

	return len(s.repositories), nil
}

func (s *Service) StartRefreshLoop(ctx context.Context, interval time.Duration) {
	s.log.WithField("interval", interval.String()).Info("starting refresh loop")

	refresh := func() {
		t0 := time.Now()

		n, err := s.Refresh(ctx)
		if err != nil {
			s.log.WithError(err).Error("refreshing repositories")
		}

		s.log.WithField("num_repos", n).WithField("refresh_duration", time.Now().Sub(t0).String()).Info("done refreshing github repositories")
	}

	refresh()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.log.Info("refreshing github repositories")
			refresh()
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) Repositories() map[string]Repository {
	s.mu.RLock()
	defer s.mu.RUnlock()

	repos := make(map[string]Repository, len(s.repositories))
	for k, v := range s.repositories {
		repos[k] = v
	}

	return repos
}

func (s *Service) Branches(ctx context.Context, repo Repository) (Repository, error) {
	branches, err := s.lister.Branches(ctx, repo)
	if err != nil {
		return Repository{}, err
	}

	repo.Branches = branches

	return repo, nil
}
