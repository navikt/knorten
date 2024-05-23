package service

import (
	"context"

	"github.com/navikt/knorten/pkg/github"
)

type GithubService interface {
	Repositories(ctx context.Context) []string
	Branches(ctx context.Context, r github.Repository) ([]string, error)
}

type githubService struct {
	ghs *github.Fetcher
}

func NewGithubService(ghs *github.Fetcher) GithubService {
	return &githubService{
		ghs: ghs,
	}
}

func (g *githubService) Repositories(_ context.Context) []string {
	repos := g.ghs.Repositories()

	var names []string
	for _, r := range repos {
		names = append(names, r.FullName)
	}

	return names
}

func (g *githubService) Branches(ctx context.Context, r github.Repository) ([]string, error) {
	repository, err := g.ghs.Branches(ctx, github.Repository{
		FullName: r.FullName,
		Name:     r.Name,
	})
	if err != nil {
		return nil, err
	}

	names := make([]string, len(repository.Branches))
	for i, r := range repository.Branches {
		names[i] = r.Name
	}

	return names, nil
}
