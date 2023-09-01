package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/k8s"
)

func TestAdminAPI(t *testing.T) {
	ctx := context.Background()
	teams, err := prepareAdminTests(ctx)
	if err != nil {
		t.Fatalf("preparing admin tests: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanUpAdminTests(ctx, teams); err != nil {
			t.Errorf("cleaning up after admin tests: %v", err)
		}
	})

	t.Run("get admin panel html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/admin", server.URL))
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

		eventsTeamA, err := repo.EventsByOwnerGet(ctx, teams[0].ID, 1)
		if err != nil {
			t.Error(err)
		}

		eventsTeamB, err := repo.EventsByOwnerGet(ctx, teams[1].ID, 1)
		if err != nil {
			t.Error(err)
		}

		expected, err := createExpectedHTML("admin/index", map[string]any{
			"teams": []teamInfo{
				{
					Team:      teams[0],
					Namespace: k8s.TeamIDToNamespace(teams[0].ID),
					Apps: []gensql.ChartType{
						gensql.ChartTypeJupyterhub,
					},
					Events: eventsTeamA,
				},
				{
					Team:      teams[1],
					Namespace: k8s.TeamIDToNamespace(teams[1].ID),
					Apps: []gensql.ChartType{
						gensql.ChartTypeJupyterhub,
						gensql.ChartTypeAirflow,
					},
					Events: eventsTeamB,
				},
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

	t.Run("get admin panel jupyter values html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/admin/jupyterhub", server.URL))
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

		expected, err := createExpectedHTML("admin/chart", map[string]any{
			"chart": string(gensql.ChartTypeJupyterhub),
			"values": []gensql.ChartGlobalValue{
				{
					Key:   "jupytervalue",
					Value: "value",
				},
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

	t.Run("update jupyter global values get confirm html", func(t *testing.T) {
		// Disable automatic redirect. For the test we need to add the session cookie to the subsequent GET request for the confirm html manually
		server.Client().CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		t.Cleanup(func() {
			server.Client().CheckRedirect = nil
		})

		data := url.Values{"jupytervalue": {"updated"}, "key.0": {"new"}, "value.0": {"new"}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/admin/jupyterhub", server.URL), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusSeeOther)
		}

		sessionCookie, err := getSessionCookieFromResponse(resp)
		if err != nil {
			t.Error(err)
		}

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%v/admin/jupyterhub/confirm", server.URL), nil)
		if err != nil {
			t.Error(err)
		}
		req.AddCookie(sessionCookie)
		resp, err = server.Client().Do(req)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		received, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}
		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Error(err)
		}

		expected, err := createExpectedHTML("admin/confirm", map[string]any{
			"chart": string(gensql.ChartTypeJupyterhub),
			"changedValues": []map[string]diffValue{
				{
					"jupytervalue": {
						Old: "value",
						New: "updated",
					},
					"new": {
						New: "new",
					},
				},
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

	t.Run("update jupyter global values", func(t *testing.T) {
		oldEvents, err := repo.EventsGetType(ctx, database.EventTypeUpdateJupyter)
		if err != nil {
			t.Error(err)
		}

		data := url.Values{"jupytervalue": {"updated"}, "new": {"new"}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/admin/jupyterhub/confirm", server.URL), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeUpdateJupyter)
		if err != nil {
			t.Error(err)
		}

		newEvents := getNewEvents(oldEvents, events)
		for _, team := range teams {
			eventPayload, err := getEventForJupyterhub(newEvents, team.ID)
			if err != nil {
				t.Error(err)
			}

			if eventPayload.TeamID == "" {
				t.Errorf("update admin values: no update jupyterhub event registered for team %v", team.ID)
			}
		}
	})

	t.Run("sync jupyterhub chart for team", func(t *testing.T) {
		oldEvents, err := repo.EventsGetType(ctx, database.EventTypeUpdateJupyter)
		if err != nil {
			t.Error(err)
		}

		data := url.Values{"team": {"team-a-1234"}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/admin/jupyterhub/sync", server.URL), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeUpdateJupyter)
		if err != nil {
			t.Error(err)
		}

		newEvents := getNewEvents(oldEvents, events)
		eventPayload, err := getEventForJupyterhub(newEvents, teams[0].ID)
		if err != nil {
			t.Error(err)
		}

		if eventPayload.TeamID == "" {
			t.Errorf("sync chart: no update jupyterhub event registered for team %v", teams[1].ID)
		}
	})

	t.Run("sync all jupyterhub charts", func(t *testing.T) {
		oldEvents, err := repo.EventsGetType(ctx, database.EventTypeUpdateJupyter)
		if err != nil {
			t.Error(err)
		}

		resp, err := server.Client().PostForm(fmt.Sprintf("%v/admin/jupyterhub/sync/all", server.URL), nil)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeUpdateJupyter)
		if err != nil {
			t.Error(err)
		}

		newEvents := getNewEvents(oldEvents, events)
		for _, team := range teams {
			eventPayload, err := getEventForJupyterhub(newEvents, team.ID)
			if err != nil {
				t.Error(err)
			}

			if eventPayload.TeamID == "" {
				t.Errorf("sync all jupyterhub charts: no update jupyterhub event registered for team %v", team.ID)
			}
		}
	})

	t.Run("get admin panel airflow values html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/admin/airflow", server.URL))
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

		expected, err := createExpectedHTML("admin/chart", map[string]any{
			"chart": string(gensql.ChartTypeAirflow),
			"values": []gensql.ChartGlobalValue{
				{
					Key:   "airflowvalue",
					Value: "value",
				},
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

	t.Run("update airflow global values", func(t *testing.T) {
		// Disable automatic redirect. For the test we need to add the session cookie to the subsequent GET request for the confirm html manually
		server.Client().CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		t.Cleanup(func() {
			server.Client().CheckRedirect = nil
		})

		data := url.Values{"airflowvalue": {"updated"}, "key.0": {"new"}, "value.0": {"new"}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/admin/airflow", server.URL), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusSeeOther)
		}

		sessionCookie, err := getSessionCookieFromResponse(resp)
		if err != nil {
			t.Error(err)
		}

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%v/admin/airflow/confirm", server.URL), nil)
		if err != nil {
			t.Error(err)
		}
		req.AddCookie(sessionCookie)
		resp, err = server.Client().Do(req)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		received, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}
		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Error(err)
		}

		expected, err := createExpectedHTML("admin/confirm", map[string]any{
			"chart": string(gensql.ChartTypeAirflow),
			"changedValues": []map[string]diffValue{
				{
					"airflowvalue": {
						Old: "value",
						New: "updated",
					},
					"new": {
						New: "new",
					},
				},
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

	t.Run("update airflow global values", func(t *testing.T) {
		oldEvents, err := repo.EventsGetType(ctx, database.EventTypeUpdateAirflow)
		if err != nil {
			t.Error(err)
		}

		data := url.Values{"airflowvalue": {"updated"}, "new": {"new"}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/admin/airflow/confirm", server.URL), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeUpdateAirflow)
		if err != nil {
			t.Error(err)
		}

		newEvents := getNewEvents(oldEvents, events)
		eventPayload, err := getEventForAirflow(newEvents, teams[1].ID)
		if err != nil {
			t.Error(err)
		}

		if eventPayload.TeamID == "" {
			t.Errorf("update airflow global values: no update airflow event registered for team %v", teams[1].ID)
		}
	})

	t.Run("sync airflow chart for team", func(t *testing.T) {
		oldEvents, err := repo.EventsGetType(ctx, database.EventTypeUpdateAirflow)
		if err != nil {
			t.Error(err)
		}

		data := url.Values{"team": {"team-b-1234"}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/admin/airflow/sync", server.URL), data)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeUpdateAirflow)
		if err != nil {
			t.Error(err)
		}

		newEvents := getNewEvents(oldEvents, events)
		eventPayload, err := getEventForAirflow(newEvents, teams[1].ID)
		if err != nil {
			t.Error(err)
		}

		if eventPayload.TeamID == "" {
			t.Errorf("sync airflow chart for team: no update airflow event registered for team %v", teams[1].ID)
		}
	})

	t.Run("sync all airflow charts", func(t *testing.T) {
		oldEvents, err := repo.EventsGetType(ctx, database.EventTypeUpdateAirflow)
		if err != nil {
			t.Error(err)
		}

		resp, err := server.Client().PostForm(fmt.Sprintf("%v/admin/airflow/sync/all", server.URL), nil)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeUpdateAirflow)
		if err != nil {
			t.Error(err)
		}

		newEvents := getNewEvents(oldEvents, events)
		eventPayload, err := getEventForAirflow(newEvents, teams[0].ID)
		if err != nil {
			t.Error(err)
		}
		if eventPayload.TeamID != "" {
			t.Errorf("sync all airflow charts: airflow event registered for team %v eventhough team does not have airflow", teams[0].ID)
		}

		eventPayload, err = getEventForAirflow(newEvents, teams[1].ID)
		if err != nil {
			t.Error(err)
		}
		if eventPayload.TeamID == "" {
			t.Errorf("sync all airflow charts: no update airflow event registered for team %v", teams[1].ID)
		}
	})

	t.Run("sync all teams", func(t *testing.T) {
		oldEvents, err := repo.EventsGetType(ctx, database.EventTypeUpdateTeam)
		if err != nil {
			t.Error(err)
		}

		resp, err := server.Client().PostForm(fmt.Sprintf("%v/admin/team/sync/all", server.URL), nil)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeUpdateTeam)
		if err != nil {
			t.Error(err)
		}

		newEvents := getNewEvents(oldEvents, events)
		for _, team := range teams {
			eventPayload, err := getEventForTeam(newEvents, team.Slug)
			if err != nil {
				t.Error(err)
			}
			if eventPayload.ID == "" {
				t.Errorf("sync all teams: no update team event registered for team %v", team.Slug)
			}
		}
	})

	t.Run("get event html", func(t *testing.T) {
		events, err := repo.EventsByOwnerGet(ctx, teams[0].ID, 1)
		if err != nil {
			t.Error(err)
		}
		if len(events) == 0 {
			t.Errorf("get event html: no event found for team %v", teams[0].ID)
		}

		resp, err := server.Client().Get(fmt.Sprintf("%v/admin/event/%v", server.URL, events[0].ID))
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

		expected, err := createExpectedHTML("admin/event", map[string]any{
			"event": events[0],
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

	t.Run("update event status", func(t *testing.T) {
		newStatus := "failed"
		events, err := repo.EventsByOwnerGet(ctx, teams[0].ID, 1)
		if err != nil {
			t.Error(err)
		}
		if len(events) == 0 {
			t.Errorf("get event html: no event found for team %v", teams[0].ID)
		}

		resp, err := server.Client().PostForm(fmt.Sprintf("%v/admin/event/%v?status=%v", server.URL, events[0].ID, newStatus), nil)
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

		events, err = repo.EventsByOwnerGet(ctx, teams[0].ID, 1)
		if err != nil {
			t.Error(err)
		}

		event := events[0]
		if newStatus != event.Status {
			t.Errorf("expected status %v, got %s", newStatus, event.Status)
		}

		expected, err := createExpectedHTML("admin/event", map[string]any{
			"event": event,
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

	t.Run("sync all compute", func(t *testing.T) {
		instance := gensql.ComputeInstance{
			Owner: user.Email,
			Name:  "compute-dummy",
		}
		if err := repo.ComputeInstanceCreate(ctx, instance); err != nil {
			t.Error(err)
		}
		t.Cleanup(func() {
			if err := repo.ComputeInstanceDelete(ctx, user.Email); err != nil {
				t.Error(err)
			}
		})

		oldEvents, err := repo.EventsGetType(ctx, database.EventTypeSyncCompute)
		if err != nil {
			t.Error(err)
		}

		resp, err := server.Client().PostForm(fmt.Sprintf("%v/admin/compute/sync/all", server.URL), nil)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status code is %v, should be %v", resp.StatusCode, http.StatusOK)
		}

		events, err := repo.EventsGetType(ctx, database.EventTypeSyncCompute)
		if err != nil {
			t.Error(err)
		}

		newEvents := getNewEvents(oldEvents, events)

		if len(newEvents) != 1 {
			t.Errorf("sync all compute: expected 1 sync event, got %v", len(newEvents))
		}

		event := newEvents[0]
		if event.Owner != instance.Owner {
			t.Errorf("sync all compute: expected event owner %v, got %v", instance.Owner, event.Owner)
		}
	})
}

func prepareAdminTests(ctx context.Context) ([]gensql.Team, error) {
	// teams
	teamA := gensql.Team{
		ID:    "team-a-1234",
		Slug:  "team-a",
		Users: []string{user.Email, "user.one@nav.no"},
	}
	err := repo.TeamCreate(ctx, teamA)
	if err != nil {
		return nil, err
	}
	if err := createChart(ctx, teamA.ID, gensql.ChartTypeJupyterhub); err != nil {
		return nil, err
	}

	teamB := gensql.Team{
		ID:    "team-b-1234",
		Slug:  "team-b",
		Users: []string{user.Email, "user.one@nav.no", "user.two@nav.no"},
	}
	err = repo.TeamCreate(ctx, teamB)
	if err != nil {
		return nil, err
	}
	if err := createChart(ctx, teamB.ID, gensql.ChartTypeJupyterhub); err != nil {
		return nil, err
	}
	if err := createChart(ctx, teamB.ID, gensql.ChartTypeAirflow); err != nil {
		return nil, err
	}

	// global values
	if err := repo.GlobalChartValueInsert(ctx, "jupytervalue", "value", false, gensql.ChartTypeJupyterhub); err != nil {
		return nil, err
	}
	if err := repo.GlobalChartValueInsert(ctx, "airflowvalue", "value", false, gensql.ChartTypeAirflow); err != nil {
		return nil, err
	}

	// events
	if err := repo.RegisterCreateTeamEvent(ctx, teamA); err != nil {
		return nil, err
	}
	if err := repo.RegisterCreateTeamEvent(ctx, teamB); err != nil {
		return nil, err
	}

	return []gensql.Team{teamA, teamB}, nil
}

func createChart(ctx context.Context, teamID string, chartType gensql.ChartType) error {
	return repo.TeamValueInsert(ctx, chartType, "dummy", "dummy", teamID)
}

func getSessionCookieFromResponse(resp *http.Response) (*http.Cookie, error) {
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "session" {
			return cookie, nil
		}
	}

	return nil, errors.New("no session cookie in http response")
}

func cleanUpAdminTests(ctx context.Context, teams []gensql.Team) error {
	for _, team := range teams {
		if err := repo.TeamDelete(ctx, team.ID); err != nil {
			return err
		}
	}

	if err := repo.GlobalValueDelete(ctx, "jupytervalue", gensql.ChartTypeJupyterhub); err != nil {
		return err
	}
	if err := repo.GlobalValueDelete(ctx, "airflowvalue", gensql.ChartTypeAirflow); err != nil {
		return err
	}

	return nil
}

func getNewEvents(oldEvents, events []gensql.Event) []gensql.Event {
	var new []gensql.Event
	for _, event := range events {
		if !containsEvent(oldEvents, event) {
			new = append(new, event)
		}
	}

	return new
}

func containsEvent(events []gensql.Event, event gensql.Event) bool {
	for _, oldEvent := range events {
		if oldEvent.ID == event.ID {
			return true
		}
	}

	return false
}
