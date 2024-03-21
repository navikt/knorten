package events

import (
	"context"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
)

type teamClient interface {
	Create(ctx context.Context, team *gensql.Team) error
	Update(ctx context.Context, team *gensql.Team) error
	Delete(ctx context.Context, teamID string) error
}

type teamMock struct {
	EventCounts map[database.EventType]int
}

func newTeamMock() teamMock {
	return teamMock{
		EventCounts: map[database.EventType]int{},
	}
}

func (tm teamMock) Create(ctx context.Context, team *gensql.Team) error {
	tm.EventCounts[database.EventTypeCreateTeam]++
	return nil
}

func (tm teamMock) Update(ctx context.Context, team *gensql.Team) error {
	tm.EventCounts[database.EventTypeUpdateTeam]++
	return nil
}

func (tm teamMock) Delete(ctx context.Context, teamID string) error {
	tm.EventCounts[database.EventTypeDeleteTeam]++
	return nil
}
