package e2etests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/nais/knorten/pkg/database/gensql"
	"golang.org/x/exp/slices"
)

func TestTeamsAPI(t *testing.T) {
	ctx := context.Background()
	testTeam := "myteam"

	t.Run("get new team html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/new", server.URL))
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

		expected, err := createExpectedHTML("team/new", nil)
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

	teamMembers := []string{"first.sirname@nav.no", "second.sirname@nav.no"}
	t.Run("create new team", func(t *testing.T) {
		data := url.Values{"team": {testTeam}, "users[]": teamMembers, "apiaccess": {""}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/new", server.URL), data)
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

		if testTeam != team.Slug {
			t.Fatalf("team slug in db should be %v, got %v", testTeam, team.Slug)
		}

		if !strings.HasPrefix(team.ID+"-", testTeam) {
			t.Fatalf("team id should have prefix %v-, got %v", testTeam, team.ID)
		}

		for _, m := range teamMembers {
			if !slices.Contains(team.Users, m) {
				t.Fatalf("team member %v not registered in db for team %v", m, testTeam)
			}
		}

		if team.ApiAccess {
			t.Fatalf("team api access should be %v, got %v", false, team.ApiAccess)
		}

		if team.RestrictAirflowEgress {
			t.Fatalf("restrict airflow egress should be %v, got %v", false, team.ApiAccess)
		}
	})

	t.Run("create new team with api access", func(t *testing.T) {
		apiAccessTeam := "apiteam"
		data := url.Values{"team": {apiAccessTeam}, "users[]": teamMembers, "apiaccess": {"on"}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/new", server.URL), data)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		team, err := repo.TeamGet(ctx, apiAccessTeam)
		if err != nil {
			t.Fatal(err)
		}

		if !team.ApiAccess {
			t.Fatalf("team api access should be %v, got %v", true, team.ApiAccess)
		}
	})

	t.Run("get edit team html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/edit", server.URL, testTeam))
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

		team, err := repo.TeamGet(ctx, testTeam)
		if err != nil {
			t.Fatal(err)
		}

		expected, err := createExpectedHTML("team/edit", map[string]any{
			"team": gensql.TeamGetRow{
				ID:    team.ID,
				Slug:  testTeam,
				Users: escape(teamMembers),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		expectedMinimized, err := minimizeHTML(expected)
		if err != nil {
			t.Fatal(err)
		}

		fmt.Println("received:")
		fmt.Println(string(received))
		fmt.Println("expected:")
		fmt.Println(expected)

		if receivedMinimized != expectedMinimized {
			t.Fatal("Received and expected HTML response are different")
		}
	})

	t.Run("get edit team html team does not exist", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/edit", server.URL, "noexist"))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("Status code is %v, should be %v", resp.StatusCode, http.StatusNotFound)
		}

		if resp.Header.Get("Content-Type") != jsonContentType {
			t.Fatalf("Content-Type header is %v, should be %v", resp.Header.Get("Content-Type"), htmlContentType)
		}
	})

	teamMembers = append(teamMembers, "third.sirname@nav.no")

	t.Run("update team", func(t *testing.T) {
		data := url.Values{"team": {testTeam}, "users[]": teamMembers, "apiaccess": {"on"}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/edit", server.URL, testTeam), data)
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

		for _, m := range teamMembers {
			if !slices.Contains(team.Users, m) {
				t.Fatalf("team member %v not registered in db for team %v", m, testTeam)
			}
		}

		if !team.ApiAccess {
			t.Fatalf("team api access should be %v, got %v", true, team.ApiAccess)
		}
	})

	t.Run("delete team", func(t *testing.T) {
		resp, err := server.Client().Post(fmt.Sprintf("%v/team/%v/delete", server.URL, testTeam), jsonContentType, nil)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		team, err := repo.TeamGet(ctx, testTeam)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				t.Fatal(err)
			}
		}

		if team.Slug == testTeam {
			t.Fatalf("team %v is not removed from db", testTeam)
		}
	})

	if err := createTeamAndApps(testTeam); err != nil {
		t.Fatalf("creating team and apps: %v", err)
	}

	t.Run("delete team verify team chart values are removed", func(t *testing.T) {
		resp, err := server.Client().Post(fmt.Sprintf("%v/team/%v/delete", server.URL, testTeam), jsonContentType, nil)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		team, err := repo.TeamGet(ctx, testTeam)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				t.Fatal(err)
			}
		}

		if team.Slug == testTeam {
			t.Fatalf("team %v is not removed from db", testTeam)
		}

		jupyterValues, err := repo.TeamValuesGet(ctx, gensql.ChartTypeJupyterhub, team.ID)
		if err != nil {
			t.Fatal(err)
		}

		if len(jupyterValues) != 0 {
			t.Fatalf("jupyter team values are not removed from db when team %v is deleted", testTeam)
		}

		airflowValues, err := repo.TeamValuesGet(ctx, gensql.ChartTypeAirflow, team.ID)
		if err != nil {
			t.Fatal(err)
		}

		if len(airflowValues) != 0 {
			t.Fatalf("airflow team values are not removed from db when team %v is deleted", testTeam)
		}
	})
}

// hvorfor må eposter i listen av brukere escape '.'?
func escape(teamMembers []string) []string {
	out := []string{}

	for _, t := range teamMembers {
		out = append(out, strings.ReplaceAll(t, ".", `\.`))
	}

	return out
}
