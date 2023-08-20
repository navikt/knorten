package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
)

func TestUserAPI(t *testing.T) {
	ctx := context.Background()
	team, err := prepareUserTests(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanupUserTests(ctx, team.ID); err != nil {
			t.Error(err)
		}
	})

	t.Run("get user html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/oversikt", server.URL))
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		if resp.Header.Get("Content-Type") != htmlContentType {
			t.Errorf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}

		received, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}
		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Error(err)
		}

		events, err := repo.EventLogsForOwnerGet(ctx, user.Email, 3)
		if err != nil {
			t.Error(err)
		}

		expected, err := createExpectedHTML("oversikt/index", map[string]any{
			"user": database.UserServices{
				Services: []database.TeamServices{
					{
						TeamID: team.ID,
						Slug:   team.Slug,
						Jupyterhub: &database.AppService{
							App:     string(gensql.ChartTypeJupyterhub),
							Ingress: fmt.Sprintf("https://%v.jupyter.knada.io", team.Slug),
							Slug:    team.Slug,
						},
						Airflow: &database.AppService{
							App:     string(gensql.ChartTypeAirflow),
							Ingress: fmt.Sprintf("https://%v.airflow.knada.io", team.Slug),
							Slug:    team.Slug,
						},
					},
				},
				Compute: &database.ComputeService{
					ComputeInstance: gensql.ComputeInstance{
						Owner: user.Email,
						Name:  "compute-" + getNormalizedNameFromEmail(user.Email),
					},
					Events: events,
				},
			},
		})
		if err != nil {
			t.Error(err)
		}
		expectedMinimized, err := minimizeHTML(expected)
		if err != nil {
			t.Error(err)
		}

		if diff := cmp.Diff(expectedMinimized, receivedMinimized); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})
}

func prepareUserTests(ctx context.Context) (*gensql.Team, error) {
	team := gensql.Team{
		ID:    "team",
		Slug:  "team",
		Users: []string{},
		Owner: user.Email,
	}
	err := repo.TeamCreate(ctx, team)
	if err != nil {
		return nil, err
	}

	if err := createChart(ctx, team.ID, gensql.ChartTypeJupyterhub); err != nil {
		return nil, err
	}

	if err := createChart(ctx, team.ID, gensql.ChartTypeAirflow); err != nil {
		return nil, err
	}

	err = repo.ComputeInstanceCreate(ctx, gensql.ComputeInstance{
		Owner: user.Email,
		Name:  "compute-" + getNormalizedNameFromEmail(user.Email),
	})
	if err != nil {
		return nil, err
	}

	return &team, nil
}

func cleanupUserTests(ctx context.Context, teamID string) error {
	if err := repo.ComputeInstanceDelete(ctx, user.Email); err != nil {
		return err
	}

	if err := repo.TeamDelete(ctx, teamID); err != nil {
		return err
	}

	return nil
}
