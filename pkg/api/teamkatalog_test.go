package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/knorten/pkg/api/service"
)

func TestTeamkatalogClient_GetActiveTeams(t *testing.T) {
	t.Run("successfully fetch and sort teams", func(t *testing.T) {
		mockResponse := service.TeamkatalogResponse{
			Content: []service.TeamkatalogTeam{
				{ID: "3", Name: "team-c"},
				{ID: "1", Name: "team-a"},
				{ID: "2", Name: "team-b"},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/team" {
				t.Errorf("Expected path '/team', got '%s'", r.URL.Path)
			}
			if r.URL.Query().Get("status") != "ACTIVE" {
				t.Errorf("Expected status=ACTIVE query param")
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		client := service.NewTeamkatalogService(server.URL)
		teams, err := client.GetActiveTeams()

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		expected := []service.TeamkatalogTeam{
			{ID: "1", Name: "team-a"},
			{ID: "2", Name: "team-b"},
			{ID: "3", Name: "team-c"},
		}

		if diff := cmp.Diff(expected, teams); diff != "" {
			t.Errorf("Teams mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("handle API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := service.NewTeamkatalogService(server.URL)
		_, err := client.GetActiveTeams()

		if err == nil {
			t.Fatal("Expected error, got nil")
		}
	})

	t.Run("handle invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		client := service.NewTeamkatalogService(server.URL)
		_, err := client.GetActiveTeams()

		if err == nil {
			t.Fatal("Expected error for invalid JSON, got nil")
		}
	})
}
