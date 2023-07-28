package e2etests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestComputeAPI(t *testing.T) {
	t.Run("get new compute html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/compute/new", server.URL))
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

		if diff := cmp.Diff(expectedMinimized, receivedMinimized); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
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

		instance, err := waitForComputeInstanceInDatabase(user.Email)
		if err != nil {
			t.Error(err)
		}

		expectedInstanceName := "compute-dummy"
		if instance.Name != expectedInstanceName {
			t.Fatalf("expected compute instance name %v, got %v", expectedInstanceName, instance.Name)
		}

		if instance.Email != user.Email {
			t.Fatalf("expected compute email to be %v, got %v", user.Email, instance.Email)
		}
	})

	t.Run("get edit compute html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/compute/edit", server.URL))
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
			"name":       "compute-dummy",
			"gcpZone":    "",
			"gcpProject": "",
		})
		if err != nil {
			t.Fatal(err)
		}

		expectedMinimized, err := minimizeHTML(expected)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(expectedMinimized, receivedMinimized); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("post delete compute", func(t *testing.T) {
		_, err := repo.ComputeInstanceGet(context.Background(), user.Email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				t.Errorf("expected compute instance to exisits in database, but it does not")
			}
			t.Error(err)
		}

		resp, err := server.Client().Post(fmt.Sprintf("%v/compute/delete", server.URL), jsonContentType, nil)
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

		timeout := 60
		for timeout > 0 {
			_, err := repo.ComputeInstanceGet(context.Background(), user.Email)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					break
				}

				t.Error(err)
			}

			time.Sleep(1 * time.Second)
			timeout--
		}

		if timeout == 0 {
			t.Errorf("timed out waiting for compute instance to be deleted")
		}
	})
}
