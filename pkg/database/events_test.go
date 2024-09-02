package database

import (
	"context"
	"testing"

	"github.com/navikt/knorten/pkg/database/gensql"
)

func TestRepo_DispatchableEventsGet(t *testing.T) {
	ctx := context.Background()

	team := gensql.Team{
		ID:    "team-a-1234",
		Slug:  "team-a",
		Users: []string{"dummy@nav.no"},
	}
	if err := repo.TeamCreate(ctx, &team); err != nil {
		t.Fatal(err)
	}
	if err := cleanupEvents(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := repo.TeamDelete(ctx, team.ID); err != nil {
			t.Fatal(err)
		}
	})

	type args struct {
		airflowEventsPaused bool
		events              []gensql.Event
	}
	tests := []struct {
		name string
		args args
		want []gensql.Event
	}{
		{
			name: "Dispatchable events verify priority",
			args: args{
				airflowEventsPaused: false,
				events: []gensql.Event{
					{
						Type:    string(EventTypeUpdateTeam),
						Payload: []byte("{}"),
						Status:  string(EventStatusPending),
						Owner:   team.ID,
					},
					{
						Type:    string(EventTypeDeleteTeam),
						Payload: []byte("{}"),
						Status:  string(EventStatusNew),
						Owner:   team.ID,
					},
					{
						Type:    string(EventTypeCreateTeam),
						Payload: []byte("{}"),
						Status:  string(EventStatusNew),
						Owner:   team.ID,
					},
				},
			},
			want: []gensql.Event{
				{
					Type:    string(EventTypeUpdateTeam),
					Payload: []byte("{}"),
					Status:  string(EventStatusPending),
					Owner:   team.ID,
				},
			},
		},
		{
			name: "Dispatchable events verify new not dispatchable when processing same type",
			args: args{
				airflowEventsPaused: false,
				events: []gensql.Event{
					{
						Type:    string(EventTypeDeleteJupyter),
						Payload: []byte("{}"),
						Status:  string(EventStatusNew),
						Owner:   team.ID,
					},
					{
						Type:    string(EventTypeUpdateJupyter),
						Payload: []byte("{}"),
						Status:  string(EventStatusProcessing),
						Owner:   team.ID,
					},
				},
			},
			want: []gensql.Event{},
		},
		{
			name: "Dispatchable events verify airflow events are excluded when airflowEventsPaused is set",
			args: args{
				airflowEventsPaused: true,
				events: []gensql.Event{
					{
						Type:    string(EventTypeCreateJupyter),
						Payload: []byte("{}"),
						Status:  string(EventStatusNew),
						Owner:   team.ID,
					},
					{
						Type:    string(EventTypeCreateAirflow),
						Payload: []byte("{}"),
						Status:  string(EventStatusNew),
						Owner:   team.ID,
					},
					{
						Type:    string(EventTypeUpdateAirflow),
						Payload: []byte("{}"),
						Status:  string(EventStatusNew),
						Owner:   team.ID,
					},
					{
						Type:    string(EventTypeHelmRolloutAirflow),
						Payload: []byte("{}"),
						Status:  string(EventStatusNew),
						Owner:   team.ID,
					},
					{
						Type:    string(EventTypeHelmRollbackAirflow),
						Payload: []byte("{}"),
						Status:  string(EventStatusNew),
						Owner:   team.ID,
					},
				},
			},
			want: []gensql.Event{
				{
					Type:   string(EventTypeCreateJupyter),
					Owner:  team.ID,
					Status: string(EventStatusNew),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := prepareEventsTest(tt.args.events); err != nil {
				t.Error(err)
			}
			t.Cleanup(func() {
				if err := cleanupEvents(); err != nil {
					t.Error(err)
				}
			})

			events, err := repo.DispatchableEventsGet(ctx, nil)
			if err != nil {
				t.Error(err)
			}

			if len(events) != len(tt.want) {
				t.Errorf("dispatchable events: expected %v events, got %v", len(tt.want), len(events))
			}

			for i, event := range events {
				if event.Type != tt.want[i].Type {
					t.Errorf("dispatchable events: expected type %v, got %v", tt.want[i].Type, event.Type)
				}
				if event.Owner != tt.want[i].Owner {
					t.Errorf("dispatchable events: expected owner %v, got %v", tt.want[i].Owner, event.Owner)
				}
				if event.Status != tt.want[i].Status {
					t.Errorf("dispatchable events: expected status %v, got %v", tt.want[i].Status, event.Status)
				}
			}
		})
	}
}

func prepareEventsTest(events []gensql.Event) error {
	for _, event := range events {
		_, err := repo.db.Exec("INSERT INTO events (owner,type,payload,deadline,status) VALUES ($1,$2,$3,$4,$5);",
			event.Owner, EventType(event.Type), event.Payload, "00:05:00", event.Status)
		if err != nil {
			return err
		}
	}

	return nil
}

func cleanupEvents() error {
	_, err := repo.db.Exec("DELETE FROM events")
	if err != nil {
		return err
	}

	return nil
}
