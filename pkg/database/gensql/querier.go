// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.20.0

package gensql

import (
	"context"

	"github.com/google/uuid"
)

type Querier interface {
	ChartDelete(ctx context.Context, arg ChartDeleteParams) error
	ChartsForTeamGet(ctx context.Context, teamID string) ([]ChartType, error)
	ComputeInstanceCreate(ctx context.Context, arg ComputeInstanceCreateParams) error
	ComputeInstanceDelete(ctx context.Context, email string) error
	ComputeInstanceGet(ctx context.Context, email string) (ComputeInstance, error)
	DispatchableEventsGet(ctx context.Context) ([]Event, error)
	DispatcherEventsProcessingGet(ctx context.Context) ([]Event, error)
	DispatcherEventsUpcomingGet(ctx context.Context) ([]Event, error)
	EventCreate(ctx context.Context, arg EventCreateParams) error
	EventGet(ctx context.Context, id uuid.UUID) (Event, error)
	EventLogCreate(ctx context.Context, arg EventLogCreateParams) error
	EventLogsForEventGet(ctx context.Context, id uuid.UUID) ([]EventLog, error)
	EventSetPendingStatus(ctx context.Context, id uuid.UUID) error
	EventSetStatus(ctx context.Context, arg EventSetStatusParams) error
	EventsByOwnerGet(ctx context.Context, arg EventsByOwnerGetParams) ([]Event, error)
	EventsGetType(ctx context.Context, eventType EventType) ([]Event, error)
	GlobalValueDelete(ctx context.Context, arg GlobalValueDeleteParams) error
	GlobalValueGet(ctx context.Context, arg GlobalValueGetParams) (ChartGlobalValue, error)
	GlobalValueInsert(ctx context.Context, arg GlobalValueInsertParams) error
	GlobalValuesGet(ctx context.Context, chartType ChartType) ([]ChartGlobalValue, error)
	SessionCreate(ctx context.Context, arg SessionCreateParams) error
	SessionDelete(ctx context.Context, token string) error
	SessionGet(ctx context.Context, token string) (Session, error)
	TeamBySlugGet(ctx context.Context, slug string) (TeamBySlugGetRow, error)
	TeamCreate(ctx context.Context, arg TeamCreateParams) error
	TeamDelete(ctx context.Context, id string) error
	TeamGet(ctx context.Context, id string) (TeamGetRow, error)
	TeamUpdate(ctx context.Context, arg TeamUpdateParams) error
	TeamValueDelete(ctx context.Context, arg TeamValueDeleteParams) error
	TeamValueGet(ctx context.Context, arg TeamValueGetParams) (ChartTeamValue, error)
	TeamValueInsert(ctx context.Context, arg TeamValueInsertParams) error
	TeamValuesGet(ctx context.Context, arg TeamValuesGetParams) ([]ChartTeamValue, error)
	TeamsForChartGet(ctx context.Context, chartType ChartType) ([]string, error)
	TeamsForUserGet(ctx context.Context, email string) ([]TeamsForUserGetRow, error)
	TeamsGet(ctx context.Context) ([]Team, error)
}

var _ Querier = (*Queries)(nil)
