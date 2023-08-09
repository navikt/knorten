package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/knorten/pkg/database/gensql"
)

func TestTeamAPI(t *testing.T) {
	ctx := context.Background()
	newTeam := "new-team"

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
			t.Fatal(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("create team: expected status code 200, got %v", resp.StatusCode)
		}

		events, err := repo.EventsGetType(ctx, gensql.EventTypeCreateTeam)
		if err != nil {
			t.Fatal(err)
		}

		eventPayload, err := getEventForTeam(events, newTeam)
		if err != nil {
			t.Fatal(err)
		}

		if eventPayload == nil {
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
			t.Fatal(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("edit team: expected status code 200, got %v", resp.StatusCode)
		}

		events, err := repo.EventsGetType(ctx, gensql.EventTypeUpdateTeam)
		if err != nil {
			t.Fatal(err)
		}

		eventPayload, err := getEventForTeam(events, existingTeam)
		if err != nil {
			t.Fatal(err)
		}

		if eventPayload == nil {
			t.Fatalf("edit team: no event registered for team %v", existingTeam)
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

		if !reflect.DeepEqual(eventPayload.Users, users) {
			t.Errorf("edit team: user list not updated, expected %v, got %v", users, eventPayload.Users)
		}
	})

	t.Run("delete team", func(t *testing.T) {
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/delete", server.URL, existingTeam), nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("delete team: expected status code 200, got %v", resp.StatusCode)
		}

		events, err := repo.EventsGetType(ctx, gensql.EventTypeDeleteTeam)
		if err != nil {
			t.Fatal(err)
		}

		if !deleteEventCreatedForTeam(events, existingTeamID) {
			t.Fatalf("delete team: no event registered for team %v", existingTeam)
		}
	})
}

func getEventForTeam(events []gensql.Event, team string) (*gensql.Team, error) {
	for _, event := range events {
		payload := &gensql.Team{}
		err := json.Unmarshal(event.Payload, payload)
		if err != nil {
			return nil, err
		}

		if payload.Slug == team {
			return payload, nil
		}
	}

	return nil, nil
}

func deleteEventCreatedForTeam(events []gensql.Event, team string) bool {
	for _, event := range events {
		if event.Owner == team {
			return true
		}
	}

	return false
}
