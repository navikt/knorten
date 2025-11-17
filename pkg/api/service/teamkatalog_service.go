package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

type TeamkatalogResponse struct {
	Content []TeamkatalogTeam `json:"content"`
}

type TeamkatalogTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TeamkatalogService interface {
	GetActiveTeams() ([]TeamkatalogTeam, error)
}

type TeamkatalogClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewTeamkatalogService(baseURL string) TeamkatalogService {
	return &TeamkatalogClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *TeamkatalogClient) GetActiveTeams() ([]TeamkatalogTeam, error) {
	url := fmt.Sprintf("%s/team?status=ACTIVE", c.baseURL)
	
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch teams from teamkatalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("teamkatalog API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response TeamkatalogResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse teamkatalog response: %w", err)
	}

	teams := response.Content
	sort.Slice(teams, func(i, j int) bool {
		return teams[i].Name < teams[j].Name
	})

	return teams, nil
}
