package helm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/navikt/knorten/pkg/database/gensql"
)

const (
	profileListKey = "singleuser.profileList"
)

func (c Client) concatenateImageProfiles(ctx context.Context, teamID string, values map[string]any) error {
	userProfileList, err := c.repo.TeamValueGet(ctx, profileListKey, teamID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	userProfiles := []map[string]any{}
	if err := json.Unmarshal([]byte(userProfileList.Value), &userProfiles); err != nil {
		return err
	}

	globalProfileList, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeJupyterhub, profileListKey)
	if err != nil {
		return err
	}
	globalProfiles := []map[string]any{}
	if err := json.Unmarshal([]byte(globalProfileList.Value), &globalProfiles); err != nil {
		return err
	}

	profiles := append(userProfiles, globalProfiles...)
	mergeMaps(values, map[string]any{
		"singleuser": map[string]any{
			"profileList": profiles,
		},
	})

	return nil
}
