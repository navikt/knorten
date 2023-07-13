package e2etests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/nais/knorten/pkg/database/gensql"
)

func TestComputeAPI(t *testing.T) {
	ctx := context.Background()
	team := gensql.Team{
		ID:    "compute-team-1234",
		Slug:  "compute-team",
		Users: []string{"bruker.en@nav.no", "bruker.to@nav.no"},
		Owner: "bruker.en@nav.no",
	}

	if err := repo.TeamCreate(ctx, team); err != nil {
		t.Fatal(err)
	}

	t.Run("get new compute html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/compute/new", server.URL, team.Slug))
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

		received, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		expected, err := createExpectedHTML("compute/new", map[string]any{})
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

	t.Run("create new compute instance", func(t *testing.T) {
		resp, err := server.Client().Post(fmt.Sprintf("%v/compute/new", server.URL), jsonContentType, nil)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		team, err := repo.TeamGet(ctx, team.Slug)
		if err != nil {
			t.Fatal(err)
		}

		instance, err := repo.ComputeInstanceGet(ctx, team.ID)
		if err != nil {
			t.Fatal(err)
		}

		expectedInstanceName := fmt.Sprintf("compute-%v", team.ID)
		if instance.Name != expectedInstanceName {
			t.Fatalf("expected compute instance name %v, got %v", expectedInstanceName, instance.Name)
		}
	})

	t.Run("get edit compute html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/compute/edit", server.URL, team.Slug))
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

		received, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		expected, err := createExpectedHTML("compute/edit", map[string]any{
			"name": team.ID,
		})
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

	if err := repo.TeamDelete(ctx, team.ID); err != nil {
		t.Fatal(err)
	}
}
