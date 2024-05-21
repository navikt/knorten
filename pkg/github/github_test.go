package github_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	ghapi "github.com/google/go-github/v62/github"
	"github.com/navikt/knorten/pkg/github"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const (
	baseURLPath = "/api-v3"
)

// Stolen from: https://github.com/google/go-github/blob/master/github/github_test.go
func setup() (c *github.Client, mux *http.ServeMux, serverURL string, teardown func()) {
	// mux is the HTTP request multiplexer used with the test server.
	mux = http.NewServeMux()

	// We want to ensure that tests catch mistakes where the endpoint URL is
	// specified as absolute rather than relative. It only makes a difference
	// when there's a non-empty base URL path. So, use that. See issue #752.
	apiHandler := http.NewServeMux()
	apiHandler.Handle(baseURLPath+"/", http.StripPrefix(baseURLPath, mux))
	apiHandler.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(os.Stderr, "FAIL: Client.BaseURL path prefix is not preserved in the request URL:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\t"+req.URL.String())
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\tDid you accidentally use an absolute endpoint URL rather than relative?")
		fmt.Fprintln(os.Stderr, "\tSee https://github.com/google/go-github/issues/752 for information.")
		http.Error(w, "Client.BaseURL path prefix is not preserved in the request URL.", http.StatusInternalServerError)
	})

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(apiHandler)

	// client is the GitHub client being tested and is
	// configured to use test server.
	client := ghapi.NewClient(nil)
	url, _ := url.Parse(server.URL + baseURLPath + "/")
	client.BaseURL = url
	client.UploadURL = url

	return github.New(github.DefaultOrganization, client), mux, server.URL, server.Close
}

func TestRepositories(t *testing.T) {
	testCases := []struct {
		name      string
		response  string
		expect    interface{}
		expectErr bool
	}{
		{
			name:     "Should return a list of repositories",
			response: `[{"name": "repo1"}, {"name": "repo2"}]`,
			expect:   []github.Repository{{Name: "repo1"}, {Name: "repo2"}},
		},
		{
			name:      "Should return an error when the response is invalid",
			response:  `invalid`,
			expectErr: true,
			expect:    "listing repositories: invalid character 'i' looking for beginning of value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, mux, _, teardown := setup()
			defer teardown()

			mux.HandleFunc(fmt.Sprintf("/orgs/%s/repos", github.DefaultOrganization), func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tc.response)
			})

			repos, err := c.Repositories(context.Background())
			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, repos)
				assert.Equal(t, tc.expect, err.Error())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, repos)
				assert.Equal(t, tc.expect, repos)
			}
		})
	}
}

func TestBranchesForRepository(t *testing.T) {
	testCases := []struct {
		name      string
		repo      string
		response  string
		expect    interface{}
		expectErr bool
	}{
		{
			name:     "Should return a list of branches",
			repo:     "repo",
			response: `[{"name": "branch1"}, {"name": "branch2"}]`,
			expect:   []github.Branch{{Name: "branch1"}, {Name: "branch2"}},
		},
		{
			name:      "Should return an error when the response is invalid",
			repo:      "repo",
			response:  `invalid`,
			expectErr: true,
			expect:    "listing branches: invalid character 'i' looking for beginning of value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, mux, _, teardown := setup()
			defer teardown()

			mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/branches", github.DefaultOrganization, tc.repo), func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tc.response)
			})

			repos, err := c.Branches(context.Background(), github.Repository{Name: tc.repo})
			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, repos)
				assert.Equal(t, tc.expect, err.Error())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, repos)
				assert.Equal(t, tc.expect, repos)
			}
		})
	}
}

func TestGithubServiceRepositories(t *testing.T) {
	testCases := []struct {
		name   string
		lister github.Lister
		expect map[string]github.Repository
	}{
		{
			name: "Should return a list of repositories",
			lister: github.NewStaticLister(
				[]github.Repository{{Name: "repo1"}, {Name: "repo2"}},
				nil,
			),
			expect: map[string]github.Repository{
				"repo1": {Name: "repo1"},
				"repo2": {Name: "repo2"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := github.NewService(tc.lister, logrus.New().WithField("subsystem", "github"))

			go s.StartRefreshLoop(context.Background(), 10*time.Second)

			time.Sleep(1 * time.Second)

			got := s.Repositories()

			assert.Equal(t, tc.expect, got)
		})
	}
}

func TestGithubServiceBranches(t *testing.T) {
	testCases := []struct {
		name   string
		lister github.Lister
		repo   github.Repository
		expect github.Repository
	}{
		{
			name: "Should return a repository with branches",
			lister: github.NewStaticLister(
				[]github.Repository{{Name: "repo1"}, {Name: "repo2"}},
				map[string][]github.Branch{
					"repo1": {{Name: "branch1"}, {Name: "branch2"}},
				},
			),
			repo: github.Repository{Name: "repo1"},
			expect: github.Repository{
				Name:     "repo1",
				Branches: []github.Branch{{Name: "branch1"}, {Name: "branch2"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := github.NewService(tc.lister, logrus.New().WithField("subsystem", "github"))

			go s.StartRefreshLoop(context.Background(), 10*time.Second)

			time.Sleep(1 * time.Second)

			got, _ := s.Branches(context.Background(), tc.repo)

			assert.Equal(t, tc.expect, got)
		})
	}
}
