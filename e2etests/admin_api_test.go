package e2etests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/nais/knorten/pkg/database/gensql"
)

func TestAdminAPI(t *testing.T) {
	teamSlug := "admintest"
	ctx := context.Background()

	t.Run("get admin html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/admin", server.URL))
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

		expected, err := createExpectedHTML("admin/index", map[string]any{
			"teams": map[string]any{},
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

	if err := createTeamAndApps(teamSlug); err != nil {
		t.Fatal(err)
	}

	t.Run("get admin html after team and app creation", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/admin", server.URL))
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

		team, err := repo.TeamBySlugGet(ctx, teamSlug)
		if err != nil {
			t.Fatal(err)
		}

		expected, err := createExpectedHTML("admin/index", map[string]any{
			"teams": map[string]any{
				team.ID: map[string]any{
					"ID":    team.ID,
					"Slug":  team.Slug,
					"Owner": "dummy@nav.no",
					"Users": []string{"annenbruker@nav.no"},
					"Apps":  []string{"jupyterhub", "airflow"},
				},
			},
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

	t.Run("get jupyterhub values html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/admin/jupyterhub", server.URL))
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

		expected, err := createExpectedHTML("admin/chart", map[string]any{
			"chart":  gensql.ChartTypeJupyterhub,
			"values": []gensql.ChartGlobalValue{},
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

	exisitingJupyterGlobals := map[string]string{
		"global.key1": "value",
		"global.key2": "value",
	}
	if err := createJupyterGlobalValues(ctx, exisitingJupyterGlobals); err != nil {
		t.Fatal(err)
	}

	t.Run("get jupyterhub values html adding", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/admin/jupyterhub", server.URL))
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

		expected, err := createExpectedHTML("admin/chart", map[string]any{
			"chart": gensql.ChartTypeJupyterhub,
			"values": []gensql.ChartGlobalValue{
				{
					Key:   "global.key1",
					Value: "value",
				},
				{
					Key:   "global.key2",
					Value: "value",
				},
			},
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

	if err := deleteJupyterGlobalValues(ctx, exisitingJupyterGlobals); err != nil {
		t.Fatal(err)
	}

	if err := cleanupTeamAndApps(teamSlug); err != nil {
		t.Fatal(err)
	}
}

func createJupyterGlobalValues(ctx context.Context, values map[string]string) error {
	for k, v := range values {
		if err := repo.GlobalChartValueInsert(ctx, k, v, false, gensql.ChartTypeJupyterhub); err != nil {
			return err
		}
	}

	return nil
}

func deleteJupyterGlobalValues(ctx context.Context, values map[string]string) error {
	for k := range values {
		if err := repo.GlobalValueDelete(ctx, k, gensql.ChartTypeJupyterhub); err != nil {
			return err
		}
	}

	return nil
}
