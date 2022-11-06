// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.15.0

package gensql

import (
	"context"
)

type Querier interface {
	GlobalValueInsert(ctx context.Context, arg GlobalValueInsertParams) error
	GlobalValuesGet(ctx context.Context, chartType ChartType) ([]ChartGlobalValue, error)
	SessionCreate(ctx context.Context, arg SessionCreateParams) error
	SessionDelete(ctx context.Context, token string) error
	SessionGet(ctx context.Context, token string) (Session, error)
	TeamValueInsert(ctx context.Context, arg TeamValueInsertParams) error
	TeamValuesGet(ctx context.Context, arg TeamValuesGetParams) ([]ChartTeamValue, error)
	UserAppInsert(ctx context.Context, arg UserAppInsertParams) error
	UserAppSetReady(ctx context.Context, arg UserAppSetReadyParams) error
	UserAppsGet(ctx context.Context, email string) ([]UserAppsGetRow, error)
}

var _ Querier = (*Queries)(nil)
