package events

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
)

type repoMock struct {
}

func (r *repoMock) EventSetStatus(ctx context.Context, id uuid.UUID, status gensql.EventStatus) error {
	return nil
}
func (r *repoMock) EventSetPendingStatus(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (r *repoMock) DispatcherEventsGet(ctx context.Context) ([]gensql.Event, error) {
	return nil, nil
}

func (r *repoMock) EventLogCreate(ctx context.Context, id uuid.UUID, message string, logType gensql.LogType) error {
	return nil
}

func TestEventHandler_distributeWork(t *testing.T) {
	type args struct {
		eventType gensql.EventType
	}
	tests := []struct {
		name string
		args args
		want workerFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := EventHandler{
				repo: &repoMock{},
			}
			if got := e.distributeWork(tt.args.eventType); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("distributeWork() = %v, want %v", got, tt.want)
			}
		})
	}
}
