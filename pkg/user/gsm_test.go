package user

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
)

func TestUserGSM(t *testing.T) {
	ctx := context.Background()
	defaultManager := gensql.UserGoogleSecretManager{
		Owner: "dummy@nav.no",
	}

	t.Cleanup(func() {
		instance, err := repo.UserGSMGet(ctx, defaultManager.Owner)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Error(err)
		}
		if err := repo.UserGSMDelete(ctx, instance.Owner); err != nil {
			t.Error(err)
		}
	})

	type args struct {
		manager gensql.UserGoogleSecretManager
	}

	type want struct {
		manager gensql.UserGoogleSecretManager
		err     error
	}

	operation := func(ctx context.Context, eventType database.EventType, manager gensql.UserGoogleSecretManager, userClient *Client) error {
		switch eventType {
		case database.EventTypeCreateUserGSM:
			return userClient.CreateUserGSM(ctx, &manager)
		case database.EventTypeDeleteUserGSM:
			return userClient.DeleteUserGSM(ctx, manager.Owner)
		}

		return nil
	}

	teamTests := []struct {
		name      string
		eventType database.EventType
		args      args
		want      want
	}{
		{
			name:      "Create User Google Secret Manager",
			eventType: database.EventTypeCreateUserGSM,
			args: args{
				manager: defaultManager,
			},
			want: want{
				manager: defaultManager,
			},
		},
		{
			name:      "Delete User Google Secret Manager",
			eventType: database.EventTypeDeleteUserGSM,
			args: args{
				manager: defaultManager,
			},
			want: want{
				manager: gensql.UserGoogleSecretManager{},
				err:     sql.ErrNoRows,
			},
		},
	}

	for _, tt := range teamTests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(repo, "", "", "", true)

			err := operation(context.Background(), tt.eventType, tt.args.manager, client)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			manager, err := repo.UserGSMGet(context.Background(), tt.args.manager.Owner)
			if !errors.Is(err, tt.want.err) {
				t.Error(err)
			}

			if diff := cmp.Diff(manager, tt.want.manager); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
