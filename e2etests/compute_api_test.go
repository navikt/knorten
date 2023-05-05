package e2etests

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/google"
)

func TestComputeAPI(t *testing.T) {
	ctx := context.Background()
	testTeam := "compute-team"

	supported, err := repo.SupportedComputeMachineTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.TeamCreate(ctx, testTeam+"-1234", testTeam, "bruker.en@nav.no", []string{"bruker.en@nav.no", "bruker.to@nav.no"}, false); err != nil {
		t.Fatal(err)
	}

	t.Run("get new compute html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/compute/new", server.URL, testTeam))
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

		expected, err := createExpectedHTML("gcp/compute", map[string]any{
			"team":          testTeam,
			"machine_types": supported,
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

	t.Run("create new compute instance", func(t *testing.T) {
		data := url.Values{"machine_type": {string(gensql.ComputeMachineTypeC2Standard4)}}
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/%v/compute/new", server.URL, testTeam), data)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status code %v, got %v", http.StatusOK, resp.StatusCode)
		}

		team, err := repo.TeamGet(ctx, testTeam)
		if err != nil {
			t.Fatal(err)
		}

		instance, err := repo.ComputeInstanceGet(ctx, team.ID)
		if err != nil {
			t.Fatal(err)
		}

		if instance.InstanceName != google.TeamToComputeInstanceName(team.ID) {
			t.Fatalf("expected compute instance name %v, got %v", google.TeamToComputeInstanceName(team.ID), instance.InstanceName)
		}

		if instance.MachineType != gensql.ComputeMachineTypeC2Standard4 {
			t.Fatalf("expected compute instance machine type %v, got %v", gensql.ComputeMachineTypeC2Standard4, instance.MachineType)
		}
	})

	t.Run("get edit compute html", func(t *testing.T) {
		resp, err := server.Client().Get(fmt.Sprintf("%v/team/%v/compute/edit", server.URL, testTeam))
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

		expected, err := createExpectedHTML("gcp/compute", map[string]any{
			"team":          testTeam,
			"machine_types": supported,
			"values": google.ComputeForm{
				Name:        "compute-" + testTeam + "-1234",
				MachineType: string(gensql.ComputeMachineTypeC2Standard4),
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

	if err := repo.TeamDelete(ctx, testTeam+"-1234"); err != nil {
		t.Fatal(err)
	}
}
