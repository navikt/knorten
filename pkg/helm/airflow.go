package helm

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
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
							"name":      "NAMESPACE",
							"valueFrom": map[string]any{"fieldRef": map[string]string{"fieldPath": "metadata.namespace"}},
						},
						{
							"name":      "ORACLE_URL",
							"valueFrom": map[string]any{"secretKeyRef": map[string]string{"name": "oracle-url", "key": "ORACLE_URL"}},
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
							"name":      "dags-data",
						},
						{
							"mountPath": "/etc/pki/tls/certs/ca-bundle.crt",
							"name":      "ca-bundle-pem",
							"readOnly":  true,
							"subPath":   "ca-bundle.pem",
						},
					},
				},
			},
		},
	}, nil
}
