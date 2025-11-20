package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/knorten/pkg/chart"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
)

func TestAirflowAPI(t *testing.T) {
	ctx := context.Background()

	team, err := prepareChartTests(ctx, "airflow-team")
	if err != nil {
		t.Errorf("preparing airflow chart tests: %v", err)
	}

	t.Cleanup(func() {
		if err := repo.TeamDelete(ctx, team.ID); err != nil {
			t.Errorf("cleaning up after airflow tests %v", err)
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

	t.Run("create new airflow", func(t *testing.T) {
		dagRepo := "navikt/repo"
		dagRepoBranch := "main"

		data := url.Values{"dagrepo": {dagRepo}, "dagrepobranch": {dagRepoBranch}, "restrictairflowegress": {""}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/airflow/new", server.URL, team.Slug), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, database.EventTypeCreateAirflow)
		if err != nil {
			t.Error(err)
		}

		eventPayload, err := getEventForAirflow(events, team.ID)
		if err != nil {
			t.Error(err)
		}

		if eventPayload.TeamID == "" {
			t.Errorf("create airflow: no event registered for team %v", team.ID)
		}

		if eventPayload.DagRepo != dagRepo {
			t.Errorf("create airflow: dag repo value, expected %v, got %v", dagRepo, eventPayload.DagRepo)
		}

		if eventPayload.DagRepoBranch != dagRepoBranch {
			t.Errorf("create airflow: dag repo branch value, expected %v, got %v", dagRepoBranch, eventPayload.DagRepoBranch)
		}

		if eventPayload.RestrictEgress {
			t.Errorf("create airflow: restrict egress value, expected %v, got %v", false, eventPayload.RestrictEgress)
		}
	})

	dagRepo := "navikt/repo"
	branch := "main"
	expectedRestrictEgress := false
	expectedValues := chart.AirflowConfigurableValues{
		TeamID:         team.ID,
		DagRepo:        dagRepo,
		DagRepoBranch:  branch,
		RestrictEgress: expectedRestrictEgress,
	}

	if err := createChartForTeam(ctx, team.ID, expectedValues, gensql.ChartTypeAirflow); err != nil {
		t.Error(err)
	}
	if err := repo.TeamChartValueInsert(ctx, chart.TeamValueKeyRestrictEgress, strconv.FormatBool(expectedRestrictEgress), team.ID, gensql.ChartTypeAirflow); err != nil {
		t.Error(err)
	}

	t.Run("get edit airflow html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/airflow/edit", server.URL, team.Slug))
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
			"values": &airflowForm{
				DagRepo:       expectedValues.DagRepo,
				DagRepoBranch: expectedValues.DagRepoBranch,
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

	t.Run("edit airflow", func(t *testing.T) {
		newDagRepo := "navikt/newrepo"
		newDagRepoBranch := "master"
		customImage := "ghcr.io/navikt/myimage:v1"

		data := url.Values{"dagrepo": {newDagRepo}, "dagrepobranch": {newDagRepoBranch}, "airflowimage": {customImage}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/airflow/edit", server.URL, team.Slug), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, database.EventTypeUpdateAirflow)
		if err != nil {
			t.Error(err)
		}

		eventPayload, err := getEventForAirflow(events, team.ID)
		if err != nil {
			t.Error(err)
		}

		if eventPayload.TeamID == "" {
			t.Errorf("edit airflow: no event registered for team %v", team.ID)
		}

		if eventPayload.DagRepo != newDagRepo {
			t.Errorf("edit airflow: dag repo value, expected %v, got %v", newDagRepo, eventPayload.DagRepo)
		}

		if eventPayload.DagRepoBranch != newDagRepoBranch {
			t.Errorf("edit airflow: dag repo branch value, expected %v, got %v", newDagRepoBranch, eventPayload.DagRepoBranch)
		}

		if strings.Join([]string{eventPayload.AirflowImage, eventPayload.AirflowTag}, ":") != customImage {
			t.Errorf("edit airflow: custom image, expected %v, got %v", customImage, strings.Join([]string{eventPayload.AirflowImage, eventPayload.AirflowTag}, ":"))
		}
	})

	t.Run("delete airflow", func(t *testing.T) {
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/airflow/delete", server.URL, team.Slug), nil)
		if err != nil {
			t.Error(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("delete team: expected status code 200, got %v", resp.StatusCode)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeDeleteAirflow)
		if err != nil {
			t.Error(err)
		}

		if !deleteEventCreatedForTeam(events, team.ID) {
			t.Errorf("delete airflow: no event registered for team %v", team.ID)
		}
	})
}

func prepareChartTests(ctx context.Context, teamName string) (gensql.Team, error) {
	team := gensql.Team{
		ID:                teamName + "-1234",
		Slug:              teamName,
		Users:             []string{testUser.Email, "user.one@nav.no", "user.two@nav.no"},
		TeamkatalogenTeam: teamName,
	}

	return team, repo.TeamCreate(ctx, &team)
}

func getEventForAirflow(events []gensql.Event, team string) (chart.AirflowConfigurableValues, error) {
	for _, event := range events {
		payload := chart.AirflowConfigurableValues{}
		err := json.Unmarshal(event.Payload, &payload)
		if err != nil {
			return chart.AirflowConfigurableValues{}, err
		}

		if payload.TeamID == team {
			return payload, nil
		}
	}

	return chart.AirflowConfigurableValues{}, nil
}

func createChartForTeam(ctx context.Context, teamID string, chartValues any, chartType gensql.ChartType) error {
	values := reflect.ValueOf(chartValues)
	for i := 0; i < values.NumField(); i++ {
		key := values.Type().Field(i).Tag.Get("helm")
		value := values.Field(i).Interface()
		if key != "" && value != "" {
			if err := repo.TeamChartValueInsert(ctx, key, value.(string), teamID, chartType); err != nil {
				return err
			}
		}
	}

	return nil
}
