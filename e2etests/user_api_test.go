package e2etests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
)

func TestOverviewAPI(t *testing.T) {
	teamSlug := "team"
	ctx := context.Background()

	t.Run("get overview html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/oversikt", server.URL))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		if resp.Header.Get("Content-Type") != htmlContentType {
			t.Fatalf("expected content type header %v, got %v", htmlContentType, resp.Header.Get("Content-Type"))
		}

		received, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		expected, err := createExpectedHTML("oversikt/index", nil)
		if err != nil {
			t.Fatal(err)
		}
		expectedMinimized, err := minimizeHTML(expected)
		if err != nil {
			t.Fatal(err)
		}

		if receivedMinimized != expectedMinimized {
			t.Fatal("Received and expected HTML response are different")
		}
	})

	if err := createTeamAndApps(teamSlug); err != nil {
		t.Fatalf("creating team and apps for overview tests: %v", err)
	}

	t.Run("get overview html after creating team and apps", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/oversikt", server.URL))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		if resp.Header.Get("Content-Type") != htmlContentType {
			t.Fatalf("expected content type header %v, got %v", htmlContentType, resp.Header.Get("Content-Type"))
		}

		received, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		team, err := repo.TeamBySlugGet(ctx, teamSlug)
		if err != nil {
			t.Fatal(err)
		}

		expected, err := createExpectedHTML("oversikt/index", map[string]any{
			"services": []database.UserServices{
				{
					Services: []database.TeamServices{
						{
							Slug:   team.Slug,
							TeamID: team.ID,
							Jupyterhub: &database.AppService{
								App:     string(gensql.ChartTypeJupyterhub),
								Ingress: "https://" + teamSlug + ".jupyter.knada.io",
								Slug:    teamSlug,
							},
							Airflow: &database.AppService{
								App:     string(gensql.ChartTypeAirflow),
								Ingress: "https://" + teamSlug + ".airflow.knada.io",
								Slug:    teamSlug,
							},
							Events: []database.Event{
								{
									ID:        uuid.New(),
									Owner:     team.ID,
									Type:      gensql.EventTypeUpdateJupyter,
									Status:    gensql.EventStatusCompleted,
									Deadline:  "30m",
									CreatedAt: time.Now(),
									UpdatedAt: time.Now(),
									Logs: []database.EventLog{
										{
											Message:   "test services",
											LogType:   gensql.LogTypeInfo,
											CreatedAt: time.Now(),
										},
									},
								},
							},
						},
					},
					Compute: &database.ComputeService{
						Email: "dummy@nav.no",
						Name:  "compute-dummy",
						Events: []database.Event{
							{
								ID:        uuid.New(),
								Owner:     team.ID,
								Type:      gensql.EventTypeCreateCompute,
								Status:    gensql.EventStatusCompleted,
								Deadline:  "30m",
								CreatedAt: time.Now(),
								UpdatedAt: time.Now(),
								Logs: []database.EventLog{
									{
										Message:   "test compute",
										LogType:   gensql.LogTypeInfo,
										CreatedAt: time.Now(),
									},
								},
							},
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		expectedMinimized, err := minimizeHTML(expected)
		if err != nil {
			t.Fatal(err)
		}

		if receivedMinimized != expectedMinimized {
			t.Fatal("Received and expected HTML response are different")
		}
	})

	if err := cleanupTeamAndApps(teamSlug); err != nil {
		t.Fatalf("cleaning up team and apps for overview tests: %v", err)
	}
}
