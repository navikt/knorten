package e2etests

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
)

func TestOverviewAPI(t *testing.T) {
	teamName := "team"
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

		received, err := ioutil.ReadAll(resp.Body)
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

	if err := createTeamAndApps(teamName); err != nil {
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

		received, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		team, err := repo.TeamGet(ctx, teamName)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := createExpectedHTML("oversikt/index", map[string]any{
			"services": []database.TeamServices{
				{
					Slug:           teamName,
					TeamID:         team.ID,
					Secret:         fmt.Sprintf("https://console.cloud.google.com/security/secret-manager/secret/%v/versions?project=knada-gcp", team.ID),
					ServiceAccount: fmt.Sprintf("%v@knada-gcp.iam.gserviceaccount.com", team.ID),
					Jupyterhub: &database.Service{
						App:     string(gensql.ChartTypeJupyterhub),
						Ingress: "https://" + teamName + ".jupyter.knada.io",
						Slug:    teamName,
					},
					Airflow: &database.Service{
						App:     string(gensql.ChartTypeAirflow),
						Ingress: "https://" + teamName + ".airflow.knada.io",
						Slug:    teamName,
					},
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		expectedMinimized, err := minimizeHTML(string(expected))
		if err != nil {
			t.Fatal(err)
		}

		if receivedMinimized != expectedMinimized {
			t.Fatal("Received and expected HTML response are different")
		}
	})

	if err := cleanupTeamAndApps(teamName); err != nil {
		t.Fatalf("cleaning up team and apps for overview tests: %v", err)
	}
}
