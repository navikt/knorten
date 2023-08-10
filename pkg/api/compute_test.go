package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/knorten/pkg/database/gensql"
)

func TestComputeAPI(t *testing.T) {
	ctx := context.Background()

	t.Run("create compute", func(t *testing.T) {
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/compute/new", server.URL), nil)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, gensql.EventTypeCreateCompute)
		if err != nil {
			t.Fatal(err)
		}

		eventPayload, err := getComputeEvent(events, user.Email)
		if err != nil {
			t.Fatal(err)
		}

		if eventPayload == nil {
			t.Errorf("create compute: no event registered for user %v", user.Email)
		}

		if eventPayload.Email != user.Email {
			t.Errorf("create compute: email expected %v, got %v", user.Email, eventPayload.Email)
		}

		if eventPayload.Name != "compute-"+getNormalizedNameFromEmail(user.Email) {
			t.Errorf("create compute: name expected %v, got %v", "compute-"+getNormalizedNameFromEmail(user.Email), eventPayload.Name)
		}
	})

	t.Run("get edit compute html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/compute/edit", server.URL))
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

		expected, err := createExpectedHTML("compute/edit", map[string]any{
			"name": "compute-" + getNormalizedNameFromEmail(user.Email),
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

	t.Run("delete compute", func(t *testing.T) {
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/compute/delete", server.URL), nil)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, gensql.EventTypeDeleteCompute)
		if err != nil {
			t.Fatal(err)
		}

		if !deleteEventCreatedForTeam(events, user.Email) {
			t.Errorf("delete compute: no event registered for user %v", user.Email)
		}
	})
}

func getComputeEvent(events []gensql.Event, user string) (*gensql.ComputeInstance, error) {
	for _, event := range events {
		payload := &gensql.ComputeInstance{}
		err := json.Unmarshal(event.Payload, payload)
		if err != nil {
			return nil, err
		}

		if payload.Email == user {
			return payload, nil
		}
	}

	return nil, nil
}
