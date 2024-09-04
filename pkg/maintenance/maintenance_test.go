package maintenance

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/knorten/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const teamID = "team-1234"

var maintenanceExclusionConfig = []*MaintenanceExclusionPeriod{
	{
		Name:  "active period",
		Team:  teamID,
		Start: time.Now(),
		End:   time.Now().Add(time.Hour * 24),
	},
	{
		Name:  "future period",
		Team:  teamID,
		Start: time.Now().Add(time.Hour * 24),
		End:   time.Now().Add(time.Hour * 48),
	},
}

func TestMaintenance(t *testing.T) {
	exclusionConfigFile, err := createExclusionConfigFile()
	if err != nil {
		t.Errorf("preparing maintenance tests: %v", err)
	}
	defer os.Remove(exclusionConfigFile.Name())

	t.Run("load maintenance exclusion config", func(t *testing.T) {
		exclusionConfig, err := LoadMaintenanceExclusionConfig(config.MaintenanceExclusionConfig{
			Enabled:  true,
			FilePath: exclusionConfigFile.Name(),
		})
		if err != nil {
			t.Fatal(err)
		}

		expected := &MaintenanceExclusion{
			Periods: map[string][]*MaintenanceExclusionPeriod{
				teamID: maintenanceExclusionConfig,
			},
		}

		diff := cmp.Diff(expected, exclusionConfig)
		assert.Empty(t, diff)
	})

	t.Run("load maintenance exclusion config enabled not set", func(t *testing.T) {
		exclusionConfig, err := LoadMaintenanceExclusionConfig(config.MaintenanceExclusionConfig{
			Enabled:  false,
			FilePath: exclusionConfigFile.Name(),
		})
		if err != nil {
			t.Fatal(err)
		}

		expected := &MaintenanceExclusion{
			Periods: map[string][]*MaintenanceExclusionPeriod{},
		}

		diff := cmp.Diff(expected, exclusionConfig)
		assert.Empty(t, diff)
	})

	t.Run("get active exclude period for team", func(t *testing.T) {
		exclusionConfig, err := LoadMaintenanceExclusionConfig(config.MaintenanceExclusionConfig{
			Enabled:  true,
			FilePath: exclusionConfigFile.Name(),
		})
		if err != nil {
			t.Fatal(err)
		}

		exclusionPeriods := exclusionConfig.ActiveExcludePeriodForTeams([]string{teamID})
		expected := []*MaintenanceExclusionPeriod{maintenanceExclusionConfig[0]}

		require.Len(t, exclusionPeriods, 1)
		diff := cmp.Diff(expected, exclusionPeriods)
		assert.Empty(t, diff)
	})

	t.Run("get all exclude periods for teams", func(t *testing.T) {
		exclusionConfig, err := LoadMaintenanceExclusionConfig(config.MaintenanceExclusionConfig{
			Enabled:  true,
			FilePath: exclusionConfigFile.Name(),
		})
		if err != nil {
			t.Fatal(err)
		}

		exclusionPeriods := exclusionConfig.ExclusionPeriodsForTeams([]string{teamID})
		expected := maintenanceExclusionConfig

		require.Len(t, exclusionPeriods, 2)
		diff := cmp.Diff(expected, exclusionPeriods)
		assert.Empty(t, diff)
	})
}

func createExclusionConfigFile() (*os.File, error) {
	file, err := os.CreateTemp("", "exclusion_config.json")
	if err != nil {
		return nil, err
	}

	exclusionConfigBytes, err := json.Marshal(maintenanceExclusionConfig)
	if err != nil {
		return nil, err
	}

	_, err = file.Write(exclusionConfigBytes)
	return file, err
}
