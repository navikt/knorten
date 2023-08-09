package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"testing"

	"github.com/nais/knorten/pkg/database/gensql"
)

func TestChartsAPI(t *testing.T) {
	ctx := context.Background()
	newTeam := "new-team"

	t.Run("create team", func(t *testing.T) {
		owner := "dummy@nav.no"
		data := url.Values{"team": {newTeam}, "owner": {owner}, "users[]": []string{}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/new", server.URL), data)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		if resp.StatusCode != 200 {
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

		if eventPayload.Owner != owner {
			t.Errorf("create team: expected owner %v, got %v", owner, eventPayload.Owner)
		}

		if len(eventPayload.Users) != 0 {
			t.Errorf("create team: expected 0 number of users, got %v", len(eventPayload.Users))
		}
	})

	existingTeam := "existing-team"
	existingTeamID := existingTeam + "-1234"
	err := repo.TeamCreate(ctx, gensql.Team{
		ID:    existingTeamID,
		Slug:  existingTeam,
		Users: []string{},
		Owner: "dummy@nav.no",
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("edit team", func(t *testing.T) {
		owner := "dummy@nav.no"
		users := []string{"user@nav.no"}
		data := url.Values{"team": {existingTeam}, "owner": {owner}, "users[]": users}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/edit", server.URL, existingTeam), data)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		if resp.StatusCode != 200 {
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
			t.Errorf("create team: expected slug %v, got %v", existingTeam, eventPayload.Slug)
		}

		if eventPayload.ID != existingTeamID {
			t.Errorf("create team: expected team id %v, got %v", existingTeamID, eventPayload.ID)
		}

		if eventPayload.Owner != owner {
			t.Errorf("create team: expected owner %v, got %v", owner, eventPayload.Owner)
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

		if resp.StatusCode != 200 {
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
