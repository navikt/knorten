package events

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type teamClient interface {
	Create(ctx context.Context, team gensql.Team, log logger.Logger) bool
	Update(ctx context.Context, team gensql.Team, log logger.Logger) bool
	Delete(ctx context.Context, teamID string, log logger.Logger) bool
}

type teamMock struct {
	EventCounts map[gensql.EventType]int
}

func newTeamMock() teamMock {
	return teamMock{
		EventCounts: map[gensql.EventType]int{},
	}
}

func (tm teamMock) Create(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	tm.EventCounts[gensql.EventTypeCreateTeam]++
	return false
}

func (tm teamMock) Update(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	tm.EventCounts[gensql.EventTypeUpdateTeam]++
	return false
}

func (tm teamMock) Delete(ctx context.Context, teamID string, log logger.Logger) bool {
	tm.EventCounts[gensql.EventTypeDeleteTeam]++
	return false
}
