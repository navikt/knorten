package e2etests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/knorten/pkg/database/gensql"
)

func TestChartsAPI(t *testing.T) {
	ctx := context.Background()
	team, err := prepareChartTests("chartteam")
	if err != nil {
		t.Fatalf("preparing chart tests: %v", err)
	}

	t.Run("get new jupyterhub html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/jupyterhub/new", server.URL, team.Slug))
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

		expected, err := createExpectedHTML("charts/jupyterhub", map[string]any{
			"team": team.Slug,
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

	t.Run("get new airflow html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/airflow/new", server.URL, team.Slug))
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

		expected, err := createExpectedHTML("charts/airflow", map[string]any{
			"team": team.Slug,
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

	t.Run("delete airflow", func(t *testing.T) {
		resp, err := server.Client().Post(fmt.Sprintf("%v/team/%v/airflow/delete", server.URL, team.Slug), jsonContentType, nil)
		if err != nil {
			t.Error(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		team, err := repo.TeamBySlugGet(ctx, team.Slug)
		if err != nil {
			t.Error(err)
		}

		airflowValues, err := repo.TeamValuesGet(ctx, gensql.ChartTypeAirflow, team.ID)
		if err != nil {
			t.Error(err)
		}

		if len(airflowValues) != 0 {
			t.Errorf("there should be no airflow team values after chart deletion, got %v values", len(airflowValues))
		}
	})

	if err := cleanupTeamAndApps(team.Slug); err != nil {
		t.Fatal(err)
	}
}

func prepareChartTests(teamSlug string) (gensql.TeamBySlugGetRow, error) {
	data := url.Values{"team": {teamSlug}, "owner": {user.Email}, "users[]": {"user.userson@nav.no"}, "apiaccess": {""}}
	resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/new", server.URL), data)
	if err != nil {
		return gensql.TeamBySlugGetRow{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return gensql.TeamBySlugGetRow{}, fmt.Errorf("creating team returned status code %v", resp.StatusCode)
	}

	return waitForTeamInDatabase(teamSlug)
}
