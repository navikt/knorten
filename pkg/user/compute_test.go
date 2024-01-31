package user

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
)

func TestCompute(t *testing.T) {
	ctx := context.Background()
	computeInstance := gensql.ComputeInstance{
		Owner: "dummy@nav.no",
		Name:  "compute-dummy",
	}
	t.Cleanup(func() {
		instance, err := repo.ComputeInstanceGet(ctx, computeInstance.Owner)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Error(err)
		}
		if err := repo.ComputeInstanceDelete(ctx, instance.Owner); err != nil {
			t.Error(err)
		}
	})
	type args struct {
		instance gensql.ComputeInstance
	}
	type want struct {
		instance gensql.ComputeInstance
		err      error
	}

	operation := func(ctx context.Context, eventType database.EventType, instance gensql.ComputeInstance, client *Client) bool {
		switch eventType {
		case database.EventTypeCreateCompute:
			return client.CreateComputeInstance(ctx, instance, logrus.NewEntry(logrus.StandardLogger()))
		case database.EventTypeDeleteCompute:
			return client.DeleteComputeInstance(ctx, instance.Owner, logrus.NewEntry(logrus.StandardLogger()))
		}

		return true
	}

	teamTests := []struct {
		name      string
		eventType database.EventType
		args      args
		want      want
	}{
		{
			name:      "Create compute instance",
			eventType: database.EventTypeCreateCompute,
			args: args{
				instance: computeInstance,
			},
			want: want{
				instance: computeInstance,
			},
		},
		{
			name:      "Delete compute instance",
			eventType: database.EventTypeDeleteCompute,
			args: args{
				instance: computeInstance,
			},
			want: want{
				instance: gensql.ComputeInstance{},
				err:      sql.ErrNoRows,
			},
		},
	}

	for _, tt := range teamTests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(repo, "", "", "", true)

			if retry := operation(context.Background(), tt.eventType, tt.args.instance, client); retry {
				t.Errorf("%v failed, got retry return for instance %v", tt.eventType, tt.args.instance.Name)
			}

			instance, err := repo.ComputeInstanceGet(context.Background(), tt.args.instance.Owner)
			if !errors.Is(err, tt.want.err) {
				t.Error(err)
			}

			if diff := cmp.Diff(instance, tt.want.instance); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
