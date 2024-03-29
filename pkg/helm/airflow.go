package helm

import (
	"context"
	"encoding/json"

	"github.com/navikt/knorten/pkg/database/gensql"
)

const (
	envKey = "env"
)

func (c Client) createKnauditInitContainer(ctx context.Context) (map[string]any, error) {
	knauditImage, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, "knauditImage,omit")
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"workers": map[string]any{
			"extraInitContainers": []map[string]any{
				{
					"name":  "knaudit",
					"image": knauditImage.Value,
					"env": []map[string]any{
						{
							"name":      "POD_NAME",
							"valueFrom": map[string]any{"fieldRef": map[string]string{"fieldPath": "metadata.name"}},
						},
						{
							"name":      "NAMESPACE",
							"valueFrom": map[string]any{"fieldRef": map[string]string{"fieldPath": "metadata.namespace"}},
						},
						{
							"name":  "KNAUDIT_PROXY_URL",
							"value": "http://knaudit-proxy.knada-system.svc.cluster.local",
						},
						{
							"name":  "CA_CERT_PATH",
							"value": "/etc/pki/tls/certs/ca-bundle.crt",
						},
						{
							"name":  "GIT_REPO_PATH",
							"value": "/dags",
						},
						{
							"name":      "AIRFLOW_DAG_ID",
							"valueFrom": map[string]any{"fieldRef": map[string]string{"fieldPath": "metadata.annotations['dag_id']"}},
						},
						{
							"name":      "AIRFLOW_RUN_ID",
							"valueFrom": map[string]any{"fieldRef": map[string]string{"fieldPath": "metadata.annotations['run_id']"}},
						},
						{
							"name":      "AIRFLOW_TASK_ID",
							"valueFrom": map[string]any{"fieldRef": map[string]string{"fieldPath": "metadata.annotations['task_id']"}},
						},
						{
							"name":      "AIRFLOW_DB_URL",
							"valueFrom": map[string]any{"secretKeyRef": map[string]string{"name": "airflow-db", "key": "connection"}},
						},
					},
					"resources": map[string]any{
						"requests": map[string]string{
							"cpu":    "200m",
							"memory": "128Mi",
						},
					},
					"volumeMounts": []map[string]any{
						{
							"mountPath": "/dags",
							"name":      "dags",
						},
						{
							"mountPath": "/etc/pki/tls/certs/ca-bundle.crt",
							"name":      "ca-bundle-pem",
							"readOnly":  true,
							"subPath":   "ca-bundle.pem",
						},
					},
					"securityContext": map[string]any{
						"allowPrivilegeEscalation": false,
						"runAsGroup":               0,
						"runAsUser":                50000,
					},
				},
			},
		},
	}, nil
}

func (c Client) concatenateCommonAirflowEnvs(ctx context.Context, teamID string, values map[string]any) error {
	globalEnvsSQL, err := c.repo.GlobalValueGet(ctx, gensql.ChartTypeAirflow, envKey)
	if err != nil {
		return err
	}
	globalEnvs := []map[string]string{}
	if err := json.Unmarshal([]byte(globalEnvsSQL.Value), &globalEnvs); err != nil {
		return err
	}

	teamEnvsSQL, err := c.repo.TeamValueGet(ctx, envKey, teamID)
	if err != nil {
		return err
	}
	teamEnvs := []map[string]string{}
	if err := json.Unmarshal([]byte(teamEnvsSQL.Value), &teamEnvs); err != nil {
		return err
	}

	mergeMaps(values, map[string]any{
		envKey: append(globalEnvs, teamEnvs...),
	})
	return nil
}
