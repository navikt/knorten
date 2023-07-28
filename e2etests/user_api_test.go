package e2etests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestOverviewAPI(t *testing.T) {
	t.Run("get overview html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/oversikt", server.URL))
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		if resp.Header.Get("Content-Type") != htmlContentType {
			t.Errorf("expected content type header %v, got %v", htmlContentType, resp.Header.Get("Content-Type"))
		}

		received, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}

		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Error(err)
		}

		services, err := repo.ServicesForUser(context.Background(), user.Email)
		if err != nil {
			t.Error(err)
		}

		expected, err := createExpectedHTML("oversikt/index", map[string]any{
			"user":       services,
			"gcpProject": "",
			"gcpZone":    "",
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
