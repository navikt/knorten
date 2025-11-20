package api

import "github.com/navikt/knorten/pkg/api/service"

type MockTeamkatalogClient struct {
	Teams []service.TeamkatalogTeam
	Error error
}

func (m *MockTeamkatalogClient) GetActiveTeams() ([]service.TeamkatalogTeam, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Teams, nil
}
