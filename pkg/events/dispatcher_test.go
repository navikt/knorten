package events

import (
	"context"
	"testing"

	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/compute"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/team"
)

func TestEventHandler_distributeWork_teamEvents(t *testing.T) {
	type args struct {
		eventType gensql.EventType
	}
	teamEventTests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "Create team event",
			args: args{
				eventType: gensql.EventTypeCreateTeam,
			},
			want: 1,
		},
		{
			name: "Update team event",
			args: args{
				eventType: gensql.EventTypeUpdateTeam,
			},
			want: 1,
		},
		{
			name: "Delete team event",
			args: args{
				eventType: gensql.EventTypeDeleteTeam,
			},
			want: 1,
		},
	}
	for _, tt := range teamEventTests {
		t.Run(tt.name, func(t *testing.T) {
			teamClientMock := team.NewTeamClientMock()
			e := EventHandler{
				repo:       &database.RepoMock{},
				teamClient: &teamClientMock,
			}
			worker := e.distributeWork(tt.args.eventType)
			if err := worker(context.Background(), gensql.Event{Payload: []byte("{}"), EventType: tt.args.eventType}, nil); err != nil {
				t.Errorf("worker(): %v", err)
			}
			if teamClientMock.EventCounts[tt.args.eventType] != tt.want {
				t.Errorf("distributeWork(): expected %v %v event, got %v", tt.want, tt.args.eventType, teamClientMock.EventCounts[tt.args.eventType])
			}
		})
	}
}

func TestEventHandler_distributeWork_computeEvents(t *testing.T) {
	type args struct {
		eventType gensql.EventType
	}
	computeEventTests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "Create compute instance event",
			args: args{
				eventType: gensql.EventTypeCreateCompute,
			},
			want: 1,
		},
		{
			name: "Delete compute instance event",
			args: args{
				eventType: gensql.EventTypeDeleteCompute,
			},
			want: 1,
		},
	}
	for _, tt := range computeEventTests {
		t.Run(tt.name, func(t *testing.T) {
			computeClientMock := compute.NewComputeClientMock()
			e := EventHandler{
				repo:          &database.RepoMock{},
				computeClient: &computeClientMock,
			}
			worker := e.distributeWork(tt.args.eventType)
			if err := worker(context.Background(), gensql.Event{Payload: []byte("{}"), EventType: tt.args.eventType}, nil); err != nil {
				t.Errorf("worker(): %v", err)
			}
			if computeClientMock.EventCounts[tt.args.eventType] != tt.want {
				t.Errorf("distributeWork(): expected %v %v event, got %v", tt.want, tt.args.eventType, computeClientMock.EventCounts[tt.args.eventType])
			}
		})
	}
}

func TestEventHandler_distributeWork_chartEvents(t *testing.T) {
	type args struct {
		eventType gensql.EventType
	}
	chartEventTests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "Create jupyterhub event",
			args: args{
				eventType: gensql.EventTypeCreateJupyter,
			},
			want: 1,
		},
		{
			name: "Update jupyterhub event",
			args: args{
				eventType: gensql.EventTypeUpdateJupyter,
			},
			want: 1,
		},
		{
			name: "Delete jupyterhub event",
			args: args{
				eventType: gensql.EventTypeDeleteJupyter,
			},
			want: 1,
		},
		{
			name: "Create airflow event",
			args: args{
				eventType: gensql.EventTypeCreateAirflow,
			},
			want: 1,
		},
		{
			name: "Update airflow event",
			args: args{
				eventType: gensql.EventTypeUpdateAirflow,
			},
			want: 1,
		},
		{
			name: "Delete airflow event",
			args: args{
				eventType: gensql.EventTypeDeleteAirflow,
			},
			want: 1,
		},
	}
	for _, tt := range chartEventTests {
		t.Run(tt.name, func(t *testing.T) {
			chartClientMock := chart.NewChartClientMock()
			e := EventHandler{
				repo:        &database.RepoMock{},
				chartClient: &chartClientMock,
			}
			worker := e.distributeWork(tt.args.eventType)
			if err := worker(context.Background(), gensql.Event{Payload: []byte("{}"), EventType: tt.args.eventType}, nil); err != nil {
				t.Errorf("worker(): %v", err)
			}
			if chartClientMock.EventCounts[tt.args.eventType] != tt.want {
				t.Errorf("distributeWork(): expected %v %v event, got %v", tt.want, tt.args.eventType, chartClientMock.EventCounts[tt.args.eventType])
			}
		})
	}
}
