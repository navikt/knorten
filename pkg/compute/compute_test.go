package compute

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/nais/knorten/local/dbsetup"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
)

var repo *database.Repo

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(filename), "../..")
	err := os.Chdir(dir)
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	var err error
	repo, err = dbsetup.SetupDBForTests()
	if err != nil {
		log.Fatal(err)
	}

	code := m.Run()
	os.Exit(code)
}

func TestCompute(t *testing.T) {
	ctx := context.Background()
	computeInstance := gensql.ComputeInstance{
		Email: "dummy@nav.no",
		Name:  "compute-dummy",
	}
	t.Cleanup(func() {
		instance, err := repo.ComputeInstanceGet(ctx, computeInstance.Email)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Error(err)
		}
		if err := repo.ComputeInstanceDelete(ctx, instance.Email); err != nil {
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

	operation := func(ctx context.Context, eventType gensql.EventType, instance gensql.ComputeInstance, computeClient *Client) bool {
		switch eventType {
		case gensql.EventTypeCreateCompute:
			return computeClient.Create(ctx, instance, logrus.NewEntry(logrus.StandardLogger()))
		case gensql.EventTypeDeleteCompute:
			return computeClient.Delete(ctx, instance.Email, logrus.NewEntry(logrus.StandardLogger()))
		}

		return true
	}

	teamTests := []struct {
		name      string
		eventType gensql.EventType
		args      args
		want      want
	}{
		{
			name:      "Create compute instance",
			eventType: gensql.EventTypeCreateCompute,
			args: args{
				instance: computeInstance,
			},
			want: want{
				instance: computeInstance,
				err:      nil,
			},
		},
		{
			name:      "Delete compute instance",
			eventType: gensql.EventTypeDeleteCompute,
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
			computeClient := NewClient(repo, "", "", true)

			if retry := operation(context.Background(), tt.eventType, tt.args.instance, computeClient); retry {
				t.Errorf("%v failed, got retry return for instance %v", tt.eventType, tt.args.instance.Name)
			}

			instance, err := repo.ComputeInstanceGet(context.Background(), tt.args.instance.Email)
			if err != tt.want.err {
				t.Error(err)
			}

			if instance.Name != tt.want.instance.Name {
				t.Errorf("instance name, expected %v, got %v", tt.want.instance.Name, instance.Name)
			}

			if instance.Email != tt.want.instance.Email {
				t.Errorf("instance email, expected %v, got %v", tt.want.instance.Email, instance.Email)
			}
		})
	}
}
