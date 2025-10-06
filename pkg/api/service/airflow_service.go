package service

import (
	"context"
)

type AirflowService interface {
	IsSchedulerDown(ctx context.Context, namespace string) (bool, error)
}
