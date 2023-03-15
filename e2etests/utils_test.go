package e2etests

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func replaceGeneratedValues(expected []byte, teamName string) ([]byte, error) {
	team, err := repo.TeamGet(context.Background(), teamName)
	if err != nil {
		return nil, err
	}

	updated := strings.ReplaceAll(string(expected), "${TEAM_ID}", team.ID)
	return []byte(updated), nil
}

func createTeamAndApps(teamName string) error {
	data := url.Values{"team": {teamName}, "users[]": {"dummy@nav.no"}, "apiaccess": {""}}
	resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/new", server.URL), data)
	if err != nil {
		return fmt.Errorf("creating team: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating team returned status code %v", resp.StatusCode)
	}

	data = url.Values{"cpu": {"1.0"}, "memory": {"1G"}, "imagename": {""}, "imagetag": {""}, "culltimeout": {"3600"}}
	resp, err = server.Client().PostForm(fmt.Sprintf("%v/team/%v/jupyterhub/new", server.URL, teamName), data)
	if err != nil {
		return fmt.Errorf("creating jupyterhub for team %v: %v", teamName, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating jupyterhub for team %v returned status code: %v", teamName, resp.StatusCode)
	}

	data = url.Values{"dagrepo": {"navikt/repo"}, "dagrepobranch": {"main"}, "apiaccess": {""}, "restrictairflowegress": {""}}
	resp, err = server.Client().PostForm(fmt.Sprintf("%v/team/%v/airflow/new", server.URL, teamName), data)
	if err != nil {
		return fmt.Errorf("creating airflow for team %v: %v", teamName, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating airflow for team %v returned status code: %v", teamName, resp.StatusCode)
	}

	return nil
}

func cleanupTeamAndApps(teamName string) error {
	resp, err := server.Client().Post(fmt.Sprintf("%v/team/%v/delete", server.URL, teamName), jsonContentType, nil)
	if err != nil {
		return fmt.Errorf("deleting team %v: %v", teamName, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deleting team returned status code %v", resp.StatusCode)
	}

	return nil
}
