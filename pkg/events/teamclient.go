package events

import (
	"context"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/logger"
)

type teamClient interface {
	Create(ctx context.Context, team gensql.Team, log logger.Logger) bool
	Update(ctx context.Context, team gensql.Team, log logger.Logger) bool
	Delete(ctx context.Context, teamID string, log logger.Logger) bool
}

type teamMock struct {
	EventCounts map[database.EventType]int
}

func newTeamMock() teamMock {
	return teamMock{
		EventCounts: map[database.EventType]int{},
	}
}

func (tm teamMock) Create(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	tm.EventCounts[database.EventTypeCreateTeam]++
	return false
}

func (tm teamMock) Update(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	tm.EventCounts[database.EventTypeUpdateTeam]++
	return false
}

func (tm teamMock) Delete(ctx context.Context, teamID string, log logger.Logger) bool {
	tm.EventCounts[database.EventTypeDeleteTeam]++
	return false
}
