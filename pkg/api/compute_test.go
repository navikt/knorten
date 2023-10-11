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
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
)

func TestComputeAPI(t *testing.T) {
	ctx := context.Background()

	t.Run("create compute", func(t *testing.T) {
		oldEvents, err := repo.EventsGetType(ctx, database.EventTypeCreateCompute)
		if err != nil {
			t.Error(err)
		}

		resp, err := server.Client().PostForm(fmt.Sprintf("%v/compute/new", server.URL), nil)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, database.EventTypeCreateCompute)
		if err != nil {
			t.Error(err)
		}

		newEvents := getNewEvents(oldEvents, events)
		eventPayload, err := getComputeEvent(newEvents, testUser.Email)
		if err != nil {
			t.Error(err)
		}

		if eventPayload.Owner == "" {
			t.Errorf("create compute: no event registered for user %v", testUser.Email)
		}

		if eventPayload.Owner != testUser.Email {
			t.Errorf("create compute: email expected %v, got %v", testUser.Email, eventPayload.Owner)
		}

		if eventPayload.Name != "compute-"+getNormalizedNameFromEmail(testUser.Email) {
			t.Errorf("create compute: name expected %v, got %v", "compute-"+getNormalizedNameFromEmail(testUser.Email), eventPayload.Name)
		}
	})

	t.Run("resize compute disk", func(t *testing.T) {
		instance := gensql.ComputeInstance{
			Owner:    testUser.Email,
			Name:     "compute-" + getNormalizedNameFromEmail(testUser.Email),
			DiskSize: "10",
		}
		if err := repo.ComputeInstanceCreate(ctx, instance); err != nil {
			t.Error(err)
		}

		t.Cleanup(func() {
			if err := repo.ComputeInstanceDelete(ctx, testUser.Email); err != nil {
				t.Error(err)
			}
		})

		oldEvents, err := repo.EventsGetType(ctx, database.EventTypeResizeCompute)
		if err != nil {
			t.Error(err)
		}

		diskSize := "200"
		data := url.Values{"diskSize": {diskSize}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/compute/edit", server.URL), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, database.EventTypeResizeCompute)
		if err != nil {
			t.Error(err)
		}

		newEvents := getNewEvents(oldEvents, events)
		eventPayload, err := getComputeEvent(newEvents, testUser.Email)
		if err != nil {
			t.Error(err)
		}

		if eventPayload.Owner == "" {
			t.Errorf("resize compute disk: no event registered for user %v", testUser.Email)
		}

		if eventPayload.Owner != testUser.Email {
			t.Errorf("resize compute disk: email expected %v, got %v", testUser.Email, eventPayload.Owner)
		}

		if eventPayload.Name != "compute-"+getNormalizedNameFromEmail(testUser.Email) {
			t.Errorf("resize compute disk: name expected %v, got %v", "compute-"+getNormalizedNameFromEmail(testUser.Email), eventPayload.Name)
		}

		if eventPayload.DiskSize != diskSize {
			t.Errorf("resize compute disk: diskSize expected %v, got %v", diskSize, eventPayload.DiskSize)
		}
	})

	t.Run("get edit compute html", func(t *testing.T) {
		instance := gensql.ComputeInstance{
			Owner:    testUser.Email,
			Name:     "compute-" + getNormalizedNameFromEmail(testUser.Email),
			DiskSize: "100",
		}
		if err := repo.ComputeInstanceCreate(ctx, instance); err != nil {
			t.Error(err)
		}

		t.Cleanup(func() {
			repo.ComputeInstanceDelete(ctx, testUser.Email)
		})

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
			"name":     instance.Name,
			"diskSize": instance.DiskSize,
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

		events, err := repo.EventsGetType(ctx, database.EventTypeDeleteCompute)
		if err != nil {
			t.Error(err)
		}

		if !deleteEventCreatedForTeam(events, testUser.Email) {
			t.Errorf("delete compute: no event registered for user %v", testUser.Email)
		}
	})
}

func getComputeEvent(events []gensql.Event, user string) (gensql.ComputeInstance, error) {
	for _, event := range events {
		payload := gensql.ComputeInstance{}
		err := json.Unmarshal(event.Payload, &payload)
		if err != nil {
			return gensql.ComputeInstance{}, err
		}

		if payload.Owner == user {
			return payload, nil
		}
	}

	return gensql.ComputeInstance{}, nil
}
