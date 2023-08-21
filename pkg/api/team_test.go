package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
)

func TestTeamAPI(t *testing.T) {
	ctx := context.Background()

	newTeam := "new-team"
	existingTeam := "existing-team"
	existingTeamID := existingTeam + "-1234"
	err := repo.TeamCreate(ctx, gensql.Team{
		ID:    existingTeamID,
		Slug:  existingTeam,
		Users: []string{},
		Owner: user.Email,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := repo.TeamDelete(ctx, existingTeamID); err != nil {
			t.Errorf("cleaning up after team tests: %v", err)
		}
	})

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

		if diff := cmp.Diff(expectedMinimized, receivedMinimized); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("create team", func(t *testing.T) {
		data := url.Values{"team": {newTeam}, "owner": {user.Email}, "users[]": []string{}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/new", server.URL), data)
		if err != nil {
			t.Error(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("create team: expected status code 200, got %v", resp.StatusCode)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeCreateTeam)
		if err != nil {
			t.Error(err)
		}

		eventPayload, err := getEventForTeam(events, newTeam)
		if err != nil {
			t.Error(err)
		}

		if eventPayload.ID == "" {
			t.Errorf("create team: no event registered for team %v", newTeam)
		}

		if eventPayload.Slug != newTeam {
			t.Errorf("create team: expected slug %v, got %v", newTeam, eventPayload.Slug)
		}

		if eventPayload.Owner != user.Email {
			t.Errorf("create team: expected owner %v, got %v", user.Email, eventPayload.Owner)
		}

		if len(eventPayload.Users) != 0 {
			t.Errorf("create team: expected 0 number of users, got %v", len(eventPayload.Users))
		}
	})

	t.Run("get edit team - team does not exist", func(t *testing.T) {
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

	t.Run("get edit team", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/edit", server.URL, existingTeam))
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

		expected, err := createExpectedHTML("team/edit", map[string]any{
			"team": gensql.TeamGetRow{
				ID:    existingTeamID,
				Slug:  existingTeam,
				Owner: user.Email,
				Users: []string{},
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

	t.Run("edit team", func(t *testing.T) {
		users := []string{"user@nav.no"}
		data := url.Values{"team": {existingTeam}, "owner": {user.Email}, "users[]": users}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/edit", server.URL, existingTeam), data)
		if err != nil {
			t.Error(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("edit team: expected status code 200, got %v", resp.StatusCode)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeUpdateTeam)
		if err != nil {
			t.Error(err)
		}

		eventPayload, err := getEventForTeam(events, existingTeam)
		if err != nil {
			t.Error(err)
		}

		if eventPayload.ID == "" {
			t.Errorf("edit team: no event registered for team %v", existingTeam)
		}

		if eventPayload.Slug != existingTeam {
			t.Errorf("edit team: expected slug %v, got %v", existingTeam, eventPayload.Slug)
		}

		if eventPayload.ID != existingTeamID {
			t.Errorf("edit team: expected team id %v, got %v", existingTeamID, eventPayload.ID)
		}

		if eventPayload.Owner != user.Email {
			t.Errorf("edit team: expected owner %v, got %v", user.Email, eventPayload.Owner)
		}

		if diff := cmp.Diff(eventPayload.Users, users); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("delete team", func(t *testing.T) {
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/delete", server.URL, existingTeam), nil)
		if err != nil {
			t.Error(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("delete team: expected status code 200, got %v", resp.StatusCode)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeDeleteTeam)
		if err != nil {
			t.Error(err)
		}

		if !deleteEventCreatedForTeam(events, existingTeamID) {
			t.Errorf("delete team: no event registered for team %v", existingTeam)
		}
	})

	t.Run("get team events", func(t *testing.T) {
		team, err := prepareTeamEventsTest(ctx)
		if err != nil {
			t.Errorf("preparing team events test: %v", err)
		}
		t.Cleanup(func() {
			if err := repo.TeamDelete(ctx, team.ID); err != nil {
				t.Errorf("deleting team in cleanup: %v", err)
			}
		})

		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/events", server.URL, team.Slug))
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

		events, err := repo.EventLogsForOwnerGet(ctx, team.ID, -1)
		if err != nil {
			t.Error(err)
		}

		if len(events) == 0 {
			t.Errorf("no events stored for team %v", team.ID)
		}

		expected, err := createExpectedHTML("team/events", map[string]any{
			"events": events,
			"slug":   team.Slug,
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
}

func getEventForTeam(events []gensql.Event, team string) (gensql.Team, error) {
	for _, event := range events {
		payload := gensql.Team{}
		err := json.Unmarshal(event.Payload, &payload)
		if err != nil {
			return gensql.Team{}, err
		}

		if payload.Slug == team {
			return payload, nil
		}
	}

	return gensql.Team{}, nil
}

func deleteEventCreatedForTeam(events []gensql.Event, team string) bool {
	for _, event := range events {
		if event.Owner == team {
			return true
		}
	}

	return false
}

func prepareTeamEventsTest(ctx context.Context) (gensql.Team, error) {
	team := gensql.Team{
		ID:    "eventtest-team-1234",
		Slug:  "eventtest-team",
		Users: []string{},
		Owner: user.Email,
	}

	if err := repo.TeamCreate(ctx, team); err != nil {
		return gensql.Team{}, err
	}

	// create events
	if err := repo.RegisterCreateJupyterEvent(ctx, team.ID, chart.JupyterConfigurableValues{}); err != nil {
		return gensql.Team{}, err
	}
	if err := repo.RegisterUpdateJupyterEvent(ctx, team.ID, chart.JupyterConfigurableValues{}); err != nil {
		return gensql.Team{}, err
	}
	if err := repo.RegisterDeleteJupyterEvent(ctx, team.ID); err != nil {
		return gensql.Team{}, err
	}

	if err := repo.RegisterCreateAirflowEvent(ctx, team.ID, chart.AirflowConfigurableValues{}); err != nil {
		return gensql.Team{}, err
	}

	return team, nil
}
