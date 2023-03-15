package e2etests

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
)

func TestUserAPI(t *testing.T) {
	teamName := "team"

	t.Run("get user html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/user", server.URL))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		if resp.Header.Get("Content-Type") != htmlContentType {
			t.Fatalf("expected content type header %v, got %v", htmlContentType, resp.Header.Get("Content-Type"))
		}

		received, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		expected, err := os.ReadFile("e2etests/testdata/html/get_user.html")
		if err != nil {
			t.Fatal(err)
		}
		expectedMinimized, err := minimizeHTML(string(expected))
		if err != nil {
			t.Fatal(err)
		}

		if receivedMinimized != expectedMinimized {
			t.Fatal("Received and expected HTML response are different")
		}
	})

	if err := createTeamAndApps(teamName); err != nil {
		t.Fatalf("creating team and apps for user tests: %v", err)
	}

	t.Run("get user html after creating team and apps", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/user", server.URL))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		if resp.Header.Get("Content-Type") != htmlContentType {
			t.Fatalf("expected content type header %v, got %v", htmlContentType, resp.Header.Get("Content-Type"))
		}

		received, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		expected, err := os.ReadFile("e2etests/testdata/html/get_user_updated.html")
		if err != nil {
			t.Fatal(err)
		}
		expectedMinimized, err := minimizeHTML(string(expected))
		if err != nil {
			t.Fatal(err)
		}

		expectedBytes, err := replaceGeneratedValues([]byte(expectedMinimized), teamName)
		if err != nil {
			t.Fatal(err)
		}

		if receivedMinimized != string(expectedBytes) {
			t.Fatal("Received and expected HTML response are different")
		}
	})

	if err := cleanupTeamAndApps(teamName); err != nil {
		t.Fatalf("cleaning up team and apps for user tests: %v", err)
	}
}
