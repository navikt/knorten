package maintenance

import (
	"encoding/json"
	"os"
	"time"

	"github.com/navikt/knorten/pkg/config"
)

type MaintenanceExclusion struct {
	Periods map[string][]*MaintenanceExclusionPeriod
}

type MaintenanceExclusionPeriod struct {
	Name  string    `json:"name"`
	Team  string    `json:"team"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

func LoadMaintenanceExclusionConfig(maintenanceExclusionConfig config.MaintenanceExclusionConfig) (*MaintenanceExclusion, error) {
	maintenanceExclusionPeriods := []*MaintenanceExclusionPeriod{}
	if maintenanceExclusionConfig.Enabled && maintenanceExclusionConfig.FilePath != "" {
		fileContentBytes, err := os.ReadFile(maintenanceExclusionConfig.FilePath)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(fileContentBytes, &maintenanceExclusionPeriods); err != nil {
			return nil, err
		}
	}

	maintenanceExclusion := &MaintenanceExclusion{Periods: map[string][]*MaintenanceExclusionPeriod{}}
	for _, mep := range maintenanceExclusionPeriods {
		maintenanceExclusion.Periods[mep.Team] = append(maintenanceExclusion.Periods[mep.Team], mep)
	}

	return maintenanceExclusion, nil
}

func (me MaintenanceExclusion) ActiveExcludePeriodForTeams(teams []string) []*MaintenanceExclusionPeriod {
	activeExcludePeriods := []*MaintenanceExclusionPeriod{}
	for _, t := range teams {
		if activeExcludePeriod := me.ActiveExcludePeriodForTeam(t); activeExcludePeriod != nil {
			activeExcludePeriods = append(activeExcludePeriods, activeExcludePeriod)
		}
	}

	return activeExcludePeriods
}

func (me MaintenanceExclusion) ActiveExcludePeriodForTeam(team string) *MaintenanceExclusionPeriod {
	today := time.Now()
	for _, period := range me.Periods[team] {
		if today.After(period.Start) && today.Before(period.End) {
			return period
		}
	}

	return nil
}

func (me MaintenanceExclusion) ExclusionPeriodsForTeams(teams []string) []*MaintenanceExclusionPeriod {
	periods := []*MaintenanceExclusionPeriod{}
	for _, team := range teams {
		if _, ok := me.Periods[team]; ok {
			periods = append(periods, me.Periods[team]...)
		}
	}

	return periods
}
