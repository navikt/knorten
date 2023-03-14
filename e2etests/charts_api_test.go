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
	"strings"
	"testing"
)

const (
	htmlContentType = "text/html; charset=utf-8"
	jsonContentType = "application/json; charset=utf-8"
	formContentType = "application/x-www-form-urlencoded"
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
			t.Fatal("Received and expected HTML response differs")
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
			t.Fatal("Received and expected HTML response differs")
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

		expectedValues, err := ioutil.ReadFile("e2etests/testdata/yaml/jupyterhub.yaml")
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

func replaceGeneratedValues(expected []byte, teamName string) ([]byte, error) {
	team, err := repo.TeamGet(context.Background(), teamName)
	if err != nil {
		return nil, err
	}

	updated := strings.ReplaceAll(string(expected), "${TEAM_ID}", team.ID)
	return []byte(updated), nil
}
