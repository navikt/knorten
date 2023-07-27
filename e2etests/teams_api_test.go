package e2etests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/knorten/pkg/database/gensql"
	"golang.org/x/exp/slices"
)

func TestTeamsAPI(t *testing.T) {
	ctx := context.Background()
	teamSlug := "myteam"
	teamMembers := []string{"first.sirname@nav.no", "second.sirname@nav.no"}
	thirdMember := "third.sirname@nav.no"

	t.Run("get new team html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/new", server.URL))
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

		expected, err := createExpectedHTML("team/new", map[string]any{
			"owner": user.Email,
		})
		if err != nil {
			t.Error(err)
		}
		expectedMinimized, err := minimizeHTML(expected)
		if err != nil {
			t.Error(err)
		}

		if receivedMinimized != expectedMinimized {
			t.Error("Received and expected HTML response are different")
		}
	})

	t.Run("create new team", func(t *testing.T) {
		data := url.Values{"team": {teamSlug}, "owner": {user.Email}, "users[]": teamMembers, "apiaccess": {""}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/new", server.URL), data)
		if err != nil {
			t.Error(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		team, err := waitForTeamInDatabase(teamSlug)
		if err != nil {
			t.Error(err)
		}

		if teamSlug != team.Slug {
			t.Errorf("team slug in db should be %v, got %v", teamSlug, team.Slug)
		}

		if !strings.HasPrefix(team.ID+"-", teamSlug) {
			t.Errorf("team id should have prefix %v-, got %v", teamSlug, team.ID)
		}

		for _, m := range teamMembers {
			if !slices.Contains(team.Users, m) {
				t.Errorf("team member %v not registered in db for team %v", m, teamSlug)
			}
		}

		if team.ApiAccess {
			t.Errorf("team api access should be %v, got %v", false, team.ApiAccess)
		}

		if team.RestrictAirflowEgress {
			t.Errorf("restrict airflow egress should be %v, got %v", false, team.ApiAccess)
		}
	})

	t.Run("create new team with api access", func(t *testing.T) {
		apiAccessTeam := "apiteam"
		data := url.Values{"team": {apiAccessTeam}, "owner": {user.Email}, "users[]": teamMembers, "apiaccess": {"on"}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/new", server.URL), data)
		if err != nil {
			t.Error(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		team, err := waitForTeamInDatabase(apiAccessTeam)
		if err != nil {
			t.Error(err)
		}

		if !team.ApiAccess {
			t.Errorf("team api access should be %v, got %v", true, team.ApiAccess)
		}

		if err := cleanupTeamAndApps(apiAccessTeam); err != nil {
			t.Error(err)
		}
	})

	t.Run("get edit team html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/edit", server.URL, teamSlug))
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

		team, err := repo.TeamBySlugGet(ctx, teamSlug)
		if err != nil {
			t.Error(err)
		}

		expected, err := createExpectedHTML("team/edit", map[string]any{
			"team": gensql.TeamGetRow{
				ID:    team.ID,
				Slug:  teamSlug,
				Owner: user.Email,
				Users: teamMembers,
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

	t.Run("get edit team html team does not exist", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/edit", server.URL, "noexist"))
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusNotFound)
		}

		if resp.Header.Get("Content-Type") != jsonContentType {
			t.Errorf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}
	})

	t.Run("update team", func(t *testing.T) {
		data := url.Values{"team": {teamSlug}, "owner": {user.Email}, "users[]": append(teamMembers, "third.sirname@nav.no"), "apiaccess": {"on"}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/edit", server.URL, teamSlug), data)
		if err != nil {
			t.Error(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		_, err = waitForTeamInDatabase(teamSlug)
		if err != nil {
			t.Error(err)
		}

		var team gensql.TeamBySlugGetRow
		timeout := 60
		for timeout > 0 {
			team, err = repo.TeamBySlugGet(context.Background(), teamSlug)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				t.Error(err)
			}

			if slices.Contains(team.Users, thirdMember) {
				break
			}

			time.Sleep(1 * time.Second)
			timeout--
		}

		if timeout == 0 {
			t.Errorf("timed out waiting for team %v to be created", teamSlug)
		}

		if !team.ApiAccess {
			t.Errorf("team api access should be %v, got %v", true, team.ApiAccess)
		}
	})

	t.Run("delete team", func(t *testing.T) {
		resp, err := server.Client().Post(fmt.Sprintf("%v/team/%v/delete", server.URL, teamSlug), jsonContentType, nil)
		if err != nil {
			t.Error(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		if err := waitForTeamToBeDeletedFromDatabase(teamSlug); err != nil {
			t.Error(err)
		}
	})

	if err := createTeamAndApps(teamSlug); err != nil {
		t.Fatalf("creating team and apps: %v", err)
	}

	t.Run("delete team verify team chart values are removed", func(t *testing.T) {
		team, err := repo.TeamBySlugGet(ctx, teamSlug)
		if err != nil {
			t.Error(err)
		}

		resp, err := server.Client().Post(fmt.Sprintf("%v/team/%v/delete", server.URL, teamSlug), jsonContentType, nil)
		if err != nil {
			t.Error(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		if err := waitForTeamToBeDeletedFromDatabase(teamSlug); err != nil {
			t.Error(err)
		}

		jupyterValues, err := repo.TeamValuesGet(ctx, gensql.ChartTypeJupyterhub, team.ID)
		if err != nil {
			t.Error(err)
		}

		if len(jupyterValues) != 0 {
			t.Errorf("jupyter team values are not removed from db when team %v is deleted", teamSlug)
		}

		airflowValues, err := repo.TeamValuesGet(ctx, gensql.ChartTypeAirflow, team.ID)
		if err != nil {
			t.Error(err)
		}

		if len(airflowValues) != 0 {
			t.Errorf("airflow team values are not removed from db when team %v is deleted", teamSlug)
		}
	})

	if err := cleanupTeamAndApps(teamSlug); err != nil {
		t.Fatal(err)
	}
}
