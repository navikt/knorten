package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/google"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
	"github.com/nais/knorten/pkg/k8s"
	"golang.org/x/exp/slices"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/database/gensql"
)

type Client struct {
	repo                *database.Repo
	k8sClient           *k8s.Client
	googleClient        *google.Google
	airflowChartVersion string
	jupyterChartVersion string
	cryptClient         *crypto.EncrypterDecrypter
	chartClient         *chart.Client
}

type diffValue struct {
	Old       string
	New       string
	Encrypted string
}

func New(repo *database.Repo, k8sClient *k8s.Client, googleClient *google.Google, cryptClient *crypto.EncrypterDecrypter, chartClient *chart.Client, airflowChartVersion, jupyterChartVersion string) *Client {
	return &Client{
		repo:                repo,
		k8sClient:           k8sClient,
		googleClient:        googleClient,
		cryptClient:         cryptClient,
		chartClient:         chartClient,
		airflowChartVersion: airflowChartVersion,
		jupyterChartVersion: jupyterChartVersion,
	}
}

func (a *Client) FindGlobalValueChanges(ctx context.Context, formValues url.Values, chartType gensql.ChartType) (map[string]diffValue, error) {
	originals, err := a.repo.GlobalValuesGet(ctx, chartType)
	if err != nil {
		return nil, err
	}

	changed := findChangedValues(originals, formValues)
	findDeletedValues(changed, originals, formValues)

	return changed, nil
}

func (a *Client) UpdateGlobalValues(ctx context.Context, formValues url.Values, chartType gensql.ChartType) error {
	for key, values := range formValues {
		if values[0] == "" {
			err := a.repo.GlobalValueDelete(ctx, key, chartType)
			if err != nil {
				return err
			}
		} else {
			value, encrypted, err := a.parseValue(values)
			if err != nil {
				return err
			}

			err = a.repo.GlobalChartValueInsert(ctx, key, value, encrypted, chartType)
			if err != nil {
				return err
			}
		}
	}

	return a.updateHelmReleases(ctx, chartType)
}

func (a *Client) ResyncTeams(ctx context.Context) error {
	teams, err := a.repo.TeamsGet(ctx)
	if err != nil {
		return err
	}

	for _, team := range teams {
		// TODO
		//if err := a.k8sClient.CreateTeamNamespace(ctx, k8s.NameToNamespace(team.ID)); err != nil {
		//	return err
		//}
		//
		//if err := a.k8sClient.CreateTeamServiceAccount(ctx, team.ID, k8s.NameToNamespace(team.ID)); err != nil {
		//	return err
		//}

		apps, err := a.repo.AppsForTeamGet(ctx, team.ID)
		if err != nil {
			return err
		}

		if slices.Contains(apps, string(gensql.ChartTypeAirflow)) {
			go a.regenerateSQLDBInfo(ctx, team.ID)
		}
	}

	return nil
}

func (a *Client) regenerateSQLDBInfo(ctx context.Context, teamID string) error {
	dbPass, err := generatePassword(40)
	if err != nil {
		return err
	}

	if err := a.k8sClient.CreateCloudSQLProxy(ctx, "airflow-sql-proxy", teamID, k8s.NameToNamespace(teamID), "airflow-"+teamID); err != nil {
		return err
	}

	dbConn := fmt.Sprintf("postgresql://%v:%v@%v:5432/%v?sslmode=disable", teamID, dbPass, "airflow-sql-proxy", teamID)
	err = a.k8sClient.CreateOrUpdateSecret(ctx, "airflow-db", k8s.NameToNamespace(teamID), map[string]string{
		"connection": dbConn,
	})
	if err != nil {
		return err
	}

	resultDBConn := fmt.Sprintf("db+postgresql://%v:%v@%v:5432/%v?sslmode=disable", teamID, dbPass, "airflow-sql-proxy", teamID)
	err = a.k8sClient.CreateOrUpdateSecret(ctx, "airflow-result-db", k8s.NameToNamespace(teamID), map[string]string{
		"connection": resultDBConn,
	})
	if err != nil {
		return err
	}

	secretKey, err := generatePassword(32)
	if err != nil {
		return err
	}

	if err := a.k8sClient.CreateOrUpdateSecret(ctx, "airflow-webserver", k8s.NameToNamespace(teamID), map[string]string{"webserver-secret-key": secretKey}); err != nil {
		return err
	}

	if err := a.googleClient.CreateOrUpdateCloudSQLUser(ctx, teamID, dbPass, "airflow-"+teamID); err != nil {
		return err
	}

	return nil
}

func (a *Client) ResyncAll(ctx context.Context, chartType gensql.ChartType) error {
	teamIDs, err := a.repo.TeamsForAppGet(ctx, chartType)
	if err != nil {
		return err
	}

	switch chartType {
	case gensql.ChartTypeJupyterhub:
		for _, tid := range teamIDs {
			if err := a.chartClient.Jupyterhub.Sync(ctx, tid); err != nil {
				return err
			}
		}
	case gensql.ChartTypeAirflow:
		for _, tid := range teamIDs {
			if err := a.chartClient.Airflow.Sync(ctx, tid); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("resyncing all instances: invalid chart type %v, err: %v", chartType, err)
	}

	return nil
}

func (a *Client) updateHelmReleases(ctx context.Context, chartType gensql.ChartType) error {
	teams, err := a.repo.TeamsForAppGet(ctx, chartType)
	if err != nil {
		return err
	}

	for _, team := range teams {
		switch chartType {
		case gensql.ChartTypeJupyterhub:
			application := helmApps.NewJupyterhub(team, a.repo, a.cryptClient, a.jupyterChartVersion)
			charty, err := application.Chart(ctx)
			if err != nil {
				return err
			}

			// Release name must be unique across namespaces as the helm chart creates a clusterrole
			// for each jupyterhub with the same name as the release name.
			releaseName := chart.JupyterReleaseName(k8s.NameToNamespace(team))
			if err := a.k8sClient.CreateHelmInstallOrUpgradeJob(ctx, team, releaseName, charty.Values); err != nil {
				return err
			}
		case gensql.ChartTypeAirflow:
			application := helmApps.NewAirflow(team, a.repo, a.cryptClient, a.airflowChartVersion)
			charty, err := application.Chart(ctx)
			if err != nil {
				return err
			}

			if err := a.k8sClient.CreateHelmInstallOrUpgradeJob(ctx, team, string(gensql.ChartTypeAirflow), charty.Values); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid chart type %v", chartType)
		}
	}

	return nil
}

func (a *Client) parseValue(values []string) (string, bool, error) {
	if len(values) == 2 {
		value, err := a.cryptClient.EncryptValue(values[0])
		if err != nil {
			return "", false, err
		}
		return value, true, nil
	}

	return values[0], false, nil
}

func findDeletedValues(changedValues map[string]diffValue, originals []gensql.ChartGlobalValue, formValues url.Values) {
	for _, original := range originals {
		notFound := true
		for key := range formValues {
			if original.Key == key {
				notFound = false
				break
			}
		}

		if notFound {
			changedValues[original.Key] = diffValue{
				Old: original.Value,
			}
		}
	}
}

func findChangedValues(originals []gensql.ChartGlobalValue, formValues url.Values) map[string]diffValue {
	changedValues := map[string]diffValue{}

	for key, values := range formValues {
		var encrypted string
		value := values[0]
		if len(values) == 2 {
			encrypted = values[1]
		}

		if strings.HasPrefix(key, "key") {
			correctValue := valueForKey(changedValues, key)
			if correctValue != nil {
				changedValues[value] = *correctValue
				delete(changedValues, key)
			} else {
				key := strings.Replace(key, "key", "value", 1)
				diff := diffValue{
					New:       key,
					Encrypted: encrypted,
				}
				changedValues[value] = diff
			}
		} else if strings.HasPrefix(key, "value") {
			correctKey := keyForValue(changedValues, key)
			if correctKey != "" {
				diff := diffValue{
					New:       value,
					Encrypted: encrypted,
				}
				changedValues[correctKey] = diff
			} else {
				key := strings.Replace(key, "value", "key", 1)
				diff := diffValue{
					New:       value,
					Encrypted: encrypted,
				}
				changedValues[key] = diff
			}
		} else {
			for _, originalValue := range originals {
				if originalValue.Key == key {
					if originalValue.Value != value {
						// TODO: Kan man endre krypterte verdier? Hvordan?
						diff := diffValue{
							Old:       originalValue.Value,
							New:       value,
							Encrypted: encrypted,
						}
						changedValues[key] = diff
						break
					}
				}
			}
		}
	}

	return changedValues
}

func valueForKey(values map[string]diffValue, needle string) *diffValue {
	for key, value := range values {
		if key == needle {
			return &value
		}
	}

	return nil
}

func keyForValue(values map[string]diffValue, needle string) string {
	for key, value := range values {
		if value.New == needle {
			return key
		}
	}

	return ""
}

func generatePassword(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
