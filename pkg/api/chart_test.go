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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database/gensql"
)

func TestJupyterAPI(t *testing.T) {
	ctx := context.Background()

	team, err := prepareChartTests(ctx, "jupyter-team")
	if err != nil {
		t.Fatalf("preparing jupyter chart tests: %v", err)
	}

	t.Cleanup(func() {
		if err := repo.TeamDelete(ctx, team.ID); err != nil {
			t.Fatalf("cleaning up after jupyter tests %v", err)
		}
	})

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

	t.Run("create new jupyterhub", func(t *testing.T) {
		cpu := "1.0"
		memory := "2G"
		culltimeout := "3600"

		data := url.Values{"cpu": {cpu}, "memory": {memory}, "imagename": {""}, "imagetag": {""}, "culltimeout": {culltimeout}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/jupyterhub/new", server.URL, team.Slug), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, gensql.EventTypeCreateJupyter)
		if err != nil {
			t.Fatal(err)
		}

		eventPayload, err := getEventForJupyterhub(events, team.ID)
		if err != nil {
			t.Fatal(err)
		}

		if eventPayload.TeamID == "" {
			t.Errorf("create jupyterhub: no event registered for team %v", team.ID)
		}
		if eventPayload.CPU != cpu {
			t.Errorf("create jupyterhub: cpu value - expected %v, got %v", cpu, eventPayload.CPU)
		}

		if eventPayload.Memory != memory {
			t.Errorf("create jupyterhub: memory value - expected %v, got %v", memory, eventPayload.Memory)
		}

		if eventPayload.CullTimeout != culltimeout {
			t.Errorf("create jupyterhub: culltimeout value - expected %v, got %v", culltimeout, eventPayload.CullTimeout)
		}

		if len(eventPayload.UserIdents) != 3 {
			t.Errorf("create jupyterhub: expected 3 users, got %v", len(eventPayload.UserIdents))
		}
	})

	expectedValues := chart.JupyterConfigurableValues{
		TeamID:      team.ID,
		CPU:         "1.0",
		Memory:      "1G",
		CullTimeout: "3600",
	}

	if err := createChartForTeam(ctx, team.ID, expectedValues, gensql.ChartTypeJupyterhub); err != nil {
		t.Fatal(err)
	}

	t.Run("get edit jupyterhub html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/jupyterhub/edit", server.URL, team.Slug))
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
			"values": &jupyterForm{
				CPU:         expectedValues.CPU,
				Memory:      expectedValues.Memory,
				CullTimeout: expectedValues.CullTimeout,
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

	t.Run("edit jupyterhub", func(t *testing.T) {
		newCPU := "2.0"
		newMemory := "2G"
		imageName := "ghcr.io/org/repo/image"
		imageTag := "v1"
		newCullTimeout := "7200"
		data := url.Values{"cpu": {newCPU}, "memory": {newMemory}, "imagename": {imageName}, "imagetag": {imageTag}, "culltimeout": {newCullTimeout}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/jupyterhub/edit", server.URL, team.Slug), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, gensql.EventTypeUpdateJupyter)
		if err != nil {
			t.Fatal(err)
		}

		eventPayload, err := getEventForJupyterhub(events, team.ID)
		if err != nil {
			t.Fatal(err)
		}

		if eventPayload.TeamID == "" {
			t.Errorf("create jupyterhub: no event registered for team %v", team.ID)
		}

		if eventPayload.CPU != newCPU {
			t.Errorf("create jupyterhub: cpu value - expected %v, got %v", newCPU, eventPayload.CPU)
		}

		if eventPayload.Memory != newMemory {
			t.Errorf("create jupyterhub: memory value - expected %v, got %v", newMemory, eventPayload.Memory)
		}

		if eventPayload.CullTimeout != newCullTimeout {
			t.Errorf("create jupyterhub: culltimeout value - expected %v, got %v", newCullTimeout, eventPayload.CullTimeout)
		}

		if eventPayload.ImageName != imageName {
			t.Errorf("create jupyterhub: image name value - expected %v, got %v", imageName, eventPayload.ImageName)
		}

		if eventPayload.ImageTag != imageTag {
			t.Errorf("create jupyterhub: image tag value - expected %v, got %v", imageTag, eventPayload.ImageTag)
		}

		if len(eventPayload.UserIdents) != 3 {
			t.Errorf("create jupyterhub: expected 3 users, got %v", len(eventPayload.UserIdents))
		}
	})

	t.Run("delete jupyterhub", func(t *testing.T) {
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/jupyterhub/delete", server.URL, team.Slug), nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("delete team: expected status code 200, got %v", resp.StatusCode)
		}

		events, err := repo.EventsGetType(ctx, gensql.EventTypeDeleteJupyter)
		if err != nil {
			t.Fatal(err)
		}

		if !deleteEventCreatedForTeam(events, team.ID) {
			t.Errorf("delete jupyterhub: no event registered for team %v", team.ID)
		}
	})
}

func TestAirflowAPI(t *testing.T) {
	ctx := context.Background()

	team, err := prepareChartTests(ctx, "airflow-team")
	if err != nil {
		t.Fatalf("preparing airflow chart tests: %v", err)
	}

	t.Cleanup(func() {
		if err := repo.TeamDelete(ctx, team.ID); err != nil {
			t.Fatalf("cleaning up after airflow tests %v", err)
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

		events, err := repo.EventsGetType(ctx, gensql.EventTypeCreateAirflow)
		if err != nil {
			t.Fatal(err)
		}

		eventPayload, err := getEventForAirflow(events, team.ID)
		if err != nil {
			t.Fatal(err)
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

	expectedRestrictEgress := false
	expectedValues := chart.AirflowConfigurableValues{
		TeamID:         team.ID,
		DagRepo:        "navikt/repo",
		DagRepoBranch:  "main",
		RestrictEgress: expectedRestrictEgress,
	}

	if err := createChartForTeam(ctx, team.ID, expectedValues, gensql.ChartTypeAirflow); err != nil {
		t.Fatal(err)
	}
	if err := repo.TeamChartValueInsert(ctx, chart.TeamValueKeyRestrictEgress, strconv.FormatBool(expectedRestrictEgress), team.ID, gensql.ChartTypeAirflow); err != nil {
		t.Fatal(err)
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

		restrictEgress := ""
		if expectedValues.RestrictEgress {
			restrictEgress = "on"
		}
		expected, err := createExpectedHTML("charts/airflow", map[string]any{
			"team": team.Slug,
			"values": &airflowForm{
				DagRepo:        expectedValues.DagRepo,
				DagRepoBranch:  expectedValues.DagRepoBranch,
				RestrictEgress: restrictEgress,
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
		restrictEgress := "on"

		data := url.Values{"dagrepo": {newDagRepo}, "dagrepobranch": {newDagRepoBranch}, "restrictairflowegress": {restrictEgress}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/airflow/edit", server.URL, team.Slug), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, gensql.EventTypeUpdateAirflow)
		if err != nil {
			t.Fatal(err)
		}

		eventPayload, err := getEventForAirflow(events, team.ID)
		if err != nil {
			t.Fatal(err)
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

		if !eventPayload.RestrictEgress {
			t.Errorf("edit airflow: restrict egress value, expected %v, got %v", true, eventPayload.RestrictEgress)
		}
	})

	t.Run("delete airflow", func(t *testing.T) {
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/airflow/delete", server.URL, team.Slug), nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("delete team: expected status code 200, got %v", resp.StatusCode)
		}

		events, err := repo.EventsGetType(ctx, gensql.EventTypeDeleteAirflow)
		if err != nil {
			t.Fatal(err)
		}

		if !deleteEventCreatedForTeam(events, team.ID) {
			t.Errorf("delete airflow: no event registered for team %v", team.ID)
		}
	})
}

func prepareChartTests(ctx context.Context, teamName string) (gensql.Team, error) {
	team := gensql.Team{
		ID:    teamName + "-1234",
		Slug:  teamName,
		Users: []string{"user.one@nav.no", "user.two@nav.no"},
		Owner: user.Email,
	}

	return team, repo.TeamCreate(ctx, team)
}

func getEventForJupyterhub(events []gensql.Event, team string) (chart.JupyterConfigurableValues, error) {
	for _, event := range events {
		payload := chart.JupyterConfigurableValues{}
		err := json.Unmarshal(event.Payload, &payload)
		if err != nil {
			return chart.JupyterConfigurableValues{}, err
		}

		if payload.TeamID == team {
			return payload, nil
		}
	}

	return chart.JupyterConfigurableValues{}, nil
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
