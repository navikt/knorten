package e2etests

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/nais/knorten/pkg/database/gensql"
)

func TestChartsAPI(t *testing.T) {
	ctx := context.Background()
	testTeam := "chartteam"
	if err := prepareChartTests(ctx, testTeam); err != nil {
		log.Fatalf("preparing chart tests: %v", err)
	}

	t.Run("get new jupyterhub html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/jupyterhub/new", server.URL, testTeam))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		if resp.Header.Get("Content-Type") != htmlContentType {
			t.Fatalf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}

		received, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		expected, err := os.ReadFile("e2etests/testdata/html/get_new_jupyterhub.html")
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

	cpu := "1.0"
	memory := "2G"
	culltimeout := "3600"

	t.Run("create new jupyterhub", func(t *testing.T) {
		data := url.Values{"cpu": {cpu}, "memory": {memory}, "imagename": {""}, "imagetag": {""}, "culltimeout": {culltimeout}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/jupyterhub/new", server.URL, testTeam), data)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		expectedValues, err := ioutil.ReadFile("e2etests/testdata/yaml/jupyterhub_new.yaml")
		if err != nil {
			t.Fatal(err)
		}
		expectedValues, err = replaceGeneratedValues(expectedValues, testTeam)
		if err != nil {
			t.Fatal(err)
		}

		actualValues, err := ioutil.ReadFile("jupyterhub.yaml")
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(expectedValues, actualValues) {
			t.Fatal("chart values out differs from expected")
		}
	})

	t.Run("get edit jupyterhub html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/jupyterhub/edit", server.URL, testTeam))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		if resp.Header.Get("Content-Type") != htmlContentType {
			t.Fatalf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}

		received, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		expected, err := os.ReadFile("e2etests/testdata/html/get_edit_jupyterhub.html")
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

	newCPU := "2.0"
	newMemory := "4G"
	newCulltimeout := "7200"

	t.Run("update jupyterhub", func(t *testing.T) {
		data := url.Values{"cpu": {newCPU}, "memory": {newMemory}, "imagename": {"ghcr.io/org/repo"}, "imagetag": {"v1"}, "culltimeout": {newCulltimeout}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/jupyterhub/edit", server.URL, testTeam), data)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		expectedValues, err := ioutil.ReadFile("e2etests/testdata/yaml/jupyterhub_updated.yaml")
		if err != nil {
			t.Fatal(err)
		}
		expectedValues, err = replaceGeneratedValues(expectedValues, testTeam)
		if err != nil {
			t.Fatal(err)
		}

		actualValues, err := ioutil.ReadFile("jupyterhub.yaml")
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(expectedValues, actualValues) {
			t.Fatal("chart values out differs from expected")
		}
	})

	t.Run("delete jupyterhub", func(t *testing.T) {
		resp, err := server.Client().Post(fmt.Sprintf("%v/team/%v/jupyterhub/delete", server.URL, testTeam), jsonContentType, nil)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		team, err := repo.TeamGet(ctx, testTeam)
		if err != nil {
			t.Fatal(err)
		}

		jupyterValues, err := repo.TeamValuesGet(ctx, gensql.ChartTypeJupyterhub, team.ID)
		if err != nil {
			t.Fatal(err)
		}

		if len(jupyterValues) != 0 {
			t.Fatalf("there should be no jupyterhub team values after chart deletion, got %v values", len(jupyterValues))
		}
	})

	t.Run("get new airflow html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/airflow/new", server.URL, testTeam))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		if resp.Header.Get("Content-Type") != htmlContentType {
			t.Fatalf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}

		received, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		expected, err := os.ReadFile("e2etests/testdata/html/get_new_airflow.html")
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

	dagRepo := "navikt/repo"
	dagRepoBranch := "main"
	apiAccess := ""
	restrictAirflowEgress := "on"

	t.Run("create new airflow", func(t *testing.T) {
		data := url.Values{"dagrepo": {dagRepo}, "dagrepobranch": {dagRepoBranch}, "apiaccess": {apiAccess}, "restrictairflowegress": {restrictAirflowEgress}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/airflow/new", server.URL, testTeam), data)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		expectedValues, err := ioutil.ReadFile("e2etests/testdata/yaml/airflow_new.yaml")
		if err != nil {
			t.Fatal(err)
		}
		expectedValues, err = replaceGeneratedValues(expectedValues, testTeam)
		if err != nil {
			t.Fatal(err)
		}

		actualValues, err := ioutil.ReadFile("airflow.yaml")
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(expectedValues, actualValues) {
			t.Fatal("chart values out differs from expected")
		}

		team, err := repo.TeamGet(ctx, testTeam)
		if err != nil {
			t.Fatal(err)
		}

		if !team.RestrictAirflowEgress {
			t.Fatalf("restrict airflow egress should be %v, got %v", true, team.RestrictAirflowEgress)
		}

		if team.ApiAccess {
			t.Fatalf("team api access should be %v, got %v", false, team.ApiAccess)
		}
	})

	t.Run("get edit airflow html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/airflow/edit", server.URL, testTeam))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		if resp.Header.Get("Content-Type") != htmlContentType {
			t.Fatalf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}

		received, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		expected, err := os.ReadFile("e2etests/testdata/html/get_edit_airflow.html")
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

	newDagRepo := "navikt/other"
	newDagRepoBranch := "dev"
	newApiAccess := "on"
	newRestrictAirflowEgress := ""

	t.Run("update airflow", func(t *testing.T) {
		data := url.Values{"dagrepo": {newDagRepo}, "dagrepobranch": {newDagRepoBranch}, "apiaccess": {newApiAccess}, "restrictairflowegress": {newRestrictAirflowEgress}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/airflow/edit", server.URL, testTeam), data)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		expectedValues, err := ioutil.ReadFile("e2etests/testdata/yaml/airflow_updated.yaml")
		if err != nil {
			t.Fatal(err)
		}
		expectedValues, err = replaceGeneratedValues(expectedValues, testTeam)
		if err != nil {
			t.Fatal(err)
		}

		actualValues, err := ioutil.ReadFile("airflow.yaml")
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(expectedValues, actualValues) {
			t.Fatal("chart values out differs from expected")
		}

		team, err := repo.TeamGet(ctx, testTeam)
		if err != nil {
			t.Fatal(err)
		}

		if team.RestrictAirflowEgress {
			t.Fatalf("restrict airflow egress should be %v, got %v", false, team.RestrictAirflowEgress)
		}

		if !team.ApiAccess {
			t.Fatalf("team api access should be %v, got %v", true, team.ApiAccess)
		}
	})

	t.Run("delete airflow", func(t *testing.T) {
		resp, err := server.Client().Post(fmt.Sprintf("%v/team/%v/airflow/delete", server.URL, testTeam), jsonContentType, nil)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		team, err := repo.TeamGet(ctx, testTeam)
		if err != nil {
			t.Fatal(err)
		}

		airflowValues, err := repo.TeamValuesGet(ctx, gensql.ChartTypeAirflow, team.ID)
		if err != nil {
			t.Fatal(err)
		}

		if len(airflowValues) != 0 {
			t.Fatalf("there should be no airflow team values after chart deletion, got %v values", len(airflowValues))
		}
	})

	t.Run("get new invalid chart", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/invalid/new", server.URL, testTeam))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("Status code is %v, should be %v", resp.StatusCode, http.StatusBadRequest)
		}

		if resp.Header.Get("Content-Type") != jsonContentType {
			t.Fatalf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}
	})

	t.Run("create new invalid chart", func(t *testing.T) {
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/invalid/new", server.URL, testTeam), url.Values{})
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status code %v, got %v", http.StatusBadRequest, resp.StatusCode)
		}

		if resp.Header.Get("Content-Type") != jsonContentType {
			t.Fatalf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}
	})

	t.Run("get edit invalid chart", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/invalid/edit", server.URL, testTeam))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status code %v, got %v", http.StatusBadRequest, resp.StatusCode)
		}

		if resp.Header.Get("Content-Type") != jsonContentType {
			t.Fatalf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}
	})

	t.Run("update invalid chart", func(t *testing.T) {
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/invalid/edit", server.URL, testTeam), url.Values{})
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status code %v, got %v", http.StatusBadRequest, resp.StatusCode)
		}

		if resp.Header.Get("Content-Type") != jsonContentType {
			t.Fatalf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}
	})

	t.Run("delete invalid chart", func(t *testing.T) {
		resp, err := server.Client().Post(fmt.Sprintf("%v/team/%v/invalid/delete", server.URL, testTeam), jsonContentType, nil)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status code %v, got %v", http.StatusBadRequest, resp.StatusCode)
		}

		if resp.Header.Get("Content-Type") != jsonContentType {
			t.Fatalf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}
	})
}

func prepareChartTests(ctx context.Context, teamName string) error {
	data := url.Values{"team": {teamName}, "users[]": {"user.userson@nav.no"}, "apiaccess": {""}}
	resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/new", server.URL), data)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating team returned status code %v", resp.StatusCode)
	}

	return nil
}
