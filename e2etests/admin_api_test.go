package e2etests

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
)

func TestAdminAPI(t *testing.T) {
	teamName := "admintest"

	if err := createTeamAndApps(teamName); err != nil {
		t.Fatal(err)
	}

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

		received, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		receivedMinimized, err := minimizeHTML(string(received))
		if err != nil {
			t.Fatal(err)
		}

		expected, err := os.ReadFile("e2etests/testdata/html/get_admin.html")
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
		t.Fatal(err)
	}
}
