-- +goose Up
DELETE FROM chart_global_values WHERE "key" = 'workers.extraInitContainers';

INSERT INTO chart_global_values ("key","value","chart_type") 
    VALUES ('knauditImage,omit','europe-north1-docker.pkg.dev/knada-gcp/knada-north/knaudit:2023-09-04-34a8e3c','airflow');

-- +goose Down
DELETE FROM chart_global_values WHERE "key" = 'knauditImage,omit'

INSERT INTO chart_global_values ("key","value","chart_type") VALUES 
    ('workers.extraInitContainers','[{"name":"knaudit","env":[{"name":"NAMESPACE","valueFrom":{"fieldRef":{"fieldPath":"metadata.namespace"}}},{"name":"ORACLE_URL","valueFrom":{"secretKeyRef":{"name":"oracle-url","key":"ORACLE_URL"}}},{"name":"CA_CERT_PATH","value":"/etc/pki/tls/certs/ca-bundle.crt"},{"name":"GIT_REPO_PATH","value":"/dags"},{"name":"AIRFLOW_DAG_ID","valueFrom":{"fieldRef":{"fieldPath":"metadata.annotations[''dag_id'']"}}},{"name":"AIRFLOW_RUN_ID","valueFrom":{"fieldRef":{"fieldPath":"metadata.annotations[''run_id'']"}}},{"name":"AIRFLOW_TASK_ID","valueFrom":{"fieldRef":{"fieldPath":"metadata.annotations[''task_id'']"}}},{"name":"AIRFLOW_DB_URL","valueFrom":{"secretKeyRef":{"name":"airflow-db","key":"connection"}}}],"image":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/knaudit:2023-09-04-34a8e3c","volumeMounts":[{"mountPath":"/dags","name":"dags-data"},{"mountPath":"/etc/pki/tls/certs/ca-bundle.crt","name":"ca-bundle-pem","readOnly":true,"subPath":"ca-bundle.pem"}]}]','airflow');
