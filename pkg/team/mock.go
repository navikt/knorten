package team

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type TeamClientMock struct {
	EventCounts map[gensql.EventType]int
}

func NewTeamClientMock() TeamClientMock {
	return TeamClientMock{
		EventCounts: map[gensql.EventType]int{},
	}
}

func (cm TeamClientMock) Create(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeCreateTeam]++
	return false
}

func (cm TeamClientMock) Update(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeUpdateTeam]++
	return false
}

func (cm TeamClientMock) Delete(ctx context.Context, teamID string, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeDeleteTeam]++
	return false
}
