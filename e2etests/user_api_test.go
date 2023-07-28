package e2etests

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/knorten/pkg/api"
	"github.com/nais/knorten/pkg/events"
	"github.com/sirupsen/logrus"
)

func TestOverviewAPI(t *testing.T) {
	eventHandler, err := events.NewHandler(context.Background(), repo, "", "", "", "", "", true, false, logrus.NewEntry(logrus.StandardLogger()))
	if err != nil {
		log.Fatalf("creating eventhandler: %v", err)
	}
	eventHandler.Run(1 * time.Second)

	srv, err := api.New(repo, true, "", "", " ", "", "nada@nav.no", "", "", logrus.NewEntry(logrus.StandardLogger()))
	if err != nil {
		log.Fatalf("creating api: %v", err)
	}

	server := httptest.NewServer(srv)

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
