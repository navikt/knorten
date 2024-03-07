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

func TestJupyterAPI(t *testing.T) {
	ctx := context.Background()

	team, err := prepareChartTests(ctx, "jupyter-team")
	if err != nil {
		t.Fatalf("preparing jupyter chart tests: %v", err)
	}

	t.Cleanup(func() {
		if err := repo.TeamDelete(ctx, team.ID); err != nil {
			t.Errorf("cleaning up after jupyter tests %v", err)
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
		cpuLimit := "1.0"
		cpuRequest := "0.5"
		memoryLimit := "2G"
		memoryRequest := "1G"
		culltimeout := "3600"
		pypiAccess := "off"

		data := url.Values{"cpulimit": {cpuLimit}, "cpurequest": {cpuRequest}, "memorylimit": {memoryLimit}, "memoryrequest": {memoryRequest}, "imagename": {""}, "imagetag": {""}, "culltimeout": {culltimeout}, "pypiaccess": {pypiAccess}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/jupyterhub/new", server.URL, team.Slug), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, database.EventTypeCreateJupyter)
		if err != nil {
			t.Error(err)
		}

		eventPayload, err := getEventForJupyterhub(events, team.ID)
		if err != nil {
			t.Error(err)
		}

		if eventPayload.TeamID == "" {
			t.Errorf("create jupyterhub: no event registered for team %v", team.ID)
		}
		if eventPayload.CPULimit != cpuLimit {
			t.Errorf("create jupyterhub: cpu value - expected %v, got %v", cpuLimit, eventPayload.CPULimit)
		}
		if eventPayload.CPURequest != cpuRequest {
			t.Errorf("create jupyterhub: cpuRequest value - expected %v, got %v", cpuRequest, eventPayload.CPURequest)
		}

		if eventPayload.MemoryLimit != memoryLimit {
			t.Errorf("create jupyterhub: memory value - expected %v, got %v", memoryLimit, eventPayload.MemoryLimit)
		}

		if eventPayload.MemoryRequest != memoryRequest {
			t.Errorf("create jupyterhub: memoryRequest value - expected %v, got %v", memoryRequest, eventPayload.MemoryRequest)
		}

		if eventPayload.CullTimeout != culltimeout {
			t.Errorf("create jupyterhub: culltimeout value - expected %v, got %v", culltimeout, eventPayload.CullTimeout)
		}

		if eventPayload.PYPIAccess {
			t.Errorf("create jupyterhub: pypiAccess value - expected %v, got %v", false, eventPayload.PYPIAccess)
		}

		if len(eventPayload.UserIdents) != 3 {
			t.Errorf("create jupyterhub: expected 3 users, got %v", len(eventPayload.UserIdents))
		}
	})

	expectedValues := chart.JupyterConfigurableValues{
		TeamID:        team.ID,
		CPULimit:      "1.0",
		CPURequest:    "0.5",
		MemoryLimit:   "1G",
		MemoryRequest: "1G",
		CullTimeout:   "3600",
	}

	if err := createChartForTeam(ctx, team.ID, expectedValues, gensql.ChartTypeJupyterhub); err != nil {
		t.Error(err)
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
				CPULimit:      expectedValues.CPULimit,
				CPURequest:    expectedValues.CPURequest,
				MemoryLimit:   expectedValues.MemoryLimit,
				MemoryRequest: expectedValues.MemoryRequest,
				CullTimeout:   expectedValues.CullTimeout,
				PYPIAccess:    "off",
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
		newCPULimit := "2.0"
		newCPURequest := "1.0"
		newMemoryLimit := "2G"
		newMemoryRequest := "0.5G"
		imageName := "ghcr.io/org/repo/image"
		imageTag := "v1"
		newCullTimeout := "7200"
		pypiAccess := "on"

		data := url.Values{"cpulimit": {newCPULimit}, "cpurequest": {newCPURequest}, "memorylimit": {newMemoryLimit}, "memoryrequest": {newMemoryRequest}, "imagename": {imageName}, "imagetag": {imageTag}, "culltimeout": {newCullTimeout}, "pypiaccess": {pypiAccess}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/jupyterhub/edit", server.URL, team.Slug), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, database.EventTypeUpdateJupyter)
		if err != nil {
			t.Error(err)
		}

		eventPayload, err := getEventForJupyterhub(events, team.ID)
		if err != nil {
			t.Error(err)
		}

		if eventPayload.TeamID == "" {
			t.Errorf("edit jupyterhub: no event registered for team %v", team.ID)
		}

		if eventPayload.CPULimit != newCPULimit {
			t.Errorf("edit jupyterhub: cpu value - expected %v, got %v", newCPULimit, eventPayload.CPULimit)
		}

		if eventPayload.CPURequest != newCPURequest {
			t.Errorf("edit jupyterhub: cpuRequest value - expected %v, got %v", newCPURequest, eventPayload.CPURequest)
		}

		if eventPayload.MemoryLimit != newMemoryLimit {
			t.Errorf("edit jupyterhub: memory value - expected %v, got %v", newMemoryLimit, eventPayload.MemoryLimit)
		}

		if eventPayload.MemoryRequest != newMemoryRequest {
			t.Errorf("edit jupyterhub: memoryRequest value - expected %v, got %v", newMemoryRequest, eventPayload.MemoryRequest)
		}

		if eventPayload.CullTimeout != newCullTimeout {
			t.Errorf("edit jupyterhub: culltimeout value - expected %v, got %v", newCullTimeout, eventPayload.CullTimeout)
		}

		if eventPayload.ImageName != imageName {
			t.Errorf("edit jupyterhub: image name value - expected %v, got %v", imageName, eventPayload.ImageName)
		}

		if eventPayload.ImageTag != imageTag {
			t.Errorf("edit jupyterhub: image tag value - expected %v, got %v", imageTag, eventPayload.ImageTag)
		}

		if !eventPayload.PYPIAccess {
			t.Errorf("edit jupyterhub: pypi access value - expected %v, got %v", true, eventPayload.PYPIAccess)
		}

		if len(eventPayload.UserIdents) != 3 {
			t.Errorf("edit jupyterhub: expected 3 users, got %v", len(eventPayload.UserIdents))
		}
	})

	t.Run("delete jupyterhub", func(t *testing.T) {
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/jupyterhub/delete", server.URL, team.Slug), nil)
		if err != nil {
			t.Error(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("delete team: expected status code 200, got %v", resp.StatusCode)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeDeleteJupyter)
		if err != nil {
			t.Error(err)
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
		ID:    teamName + "-1234",
		Slug:  teamName,
		Users: []string{testUser.Email, "user.one@nav.no", "user.two@nav.no"},
	}

	return team, repo.TeamCreate(ctx, &team)
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
