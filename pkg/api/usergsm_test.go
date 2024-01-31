package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
)

func TestUserGSMAPI(t *testing.T) {
	ctx := context.Background()

	t.Run("create User Google Secret Manager", func(t *testing.T) {
		oldEvents, err := repo.EventsGetType(ctx, database.EventTypeCreateUserGSM)
		if err != nil {
			t.Error(err)
		}

		resp, err := server.Client().PostForm(fmt.Sprintf("%v/secret/new", server.URL), nil)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, database.EventTypeCreateUserGSM)
		if err != nil {
			t.Error(err)
		}

		newEvents := getNewEvents(oldEvents, events)
		secretManager, err := getUserGSMEvent(newEvents, testUser.Email)
		if err != nil {
			t.Error(err)
		}

		want := gensql.UserGoogleSecretManager{
			Owner: testUser.Email,
			Name:  getNormalizedNameFromEmail(testUser.Email),
		}
		if diff := cmp.Diff(want, secretManager); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("delete User Google Secret Manager", func(t *testing.T) {
		resp, err := server.Client().PostForm(fmt.Sprintf("%v/secret/delete", server.URL), nil)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		events, err := repo.EventsGetType(ctx, database.EventTypeDeleteUserGSM)
		if err != nil {
			t.Error(err)
		}

		if !deleteEventCreatedForTeam(events, testUser.Email) {
			t.Errorf("delete secret: no event registered for user %v", testUser.Email)
		}
	})
}

func getUserGSMEvent(events []gensql.Event, user string) (gensql.UserGoogleSecretManager, error) {
	for _, event := range events {
		payload := gensql.UserGoogleSecretManager{}
		err := json.Unmarshal(event.Payload, &payload)
		if err != nil {
			return gensql.UserGoogleSecretManager{}, err
		}

		if payload.Owner == user {
			return payload, nil
		}
	}

	return gensql.UserGoogleSecretManager{}, nil
}
