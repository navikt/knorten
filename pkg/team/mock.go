package team

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type ClientMock struct {
	EventCounts map[gensql.EventType]int
}

func NewClientMock() ClientMock {
	return ClientMock{
		EventCounts: map[gensql.EventType]int{},
	}
}

func (cm ClientMock) Create(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeCreateTeam]++
	return false
}

func (cm ClientMock) Update(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeUpdateTeam]++
	return false
}

func (cm ClientMock) Delete(ctx context.Context, teamID string, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeDeleteTeam]++
	return false
}
