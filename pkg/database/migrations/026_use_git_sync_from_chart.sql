-- +goose Up
DELETE FROM chart_global_values WHERE "key" IN ('scheduler.extraContainers','scheduler.extraInitContainers','webserver.extraContainers','workers.extraInitContainers');
DELETE FROM chart_global_values WHERE "key" IN ('scheduler.extraVolumes','scheduler.extraVolumeMounts','workers.extraVolumes','workers.extraVolumeMounts','webserver.extraVolumes','webserver.extraVolumeMounts');

INSERT INTO chart_global_values ("key","value","chart_type") VALUES
    ('dags.gitSync.enabled','true','airflow'),
    ('dags.gitSync.extraVolumeMounts','[{"mountPath":"/dags","name":"dags-data"},{"mountPath":"/keys","name":"github-app-secret"}]','airflow'),
    ('images.gitSync.repository','europe-north1-docker.pkg.dev/knada-gcp/knada-north/git-sync','airflow'),
    ('images.gitSync.tag','2023-09-18-2c62c53','airflow');

INSERT INTO chart_global_values ("key","value","chart_type") VALUES 
    ('workers.extraInitContainers','[{"name":"knaudit","env":[{"name":"NAMESPACE","valueFrom":{"fieldRef":{"fieldPath":"metadata.namespace"}}},{"name":"ORACLE_URL","valueFrom":{"secretKeyRef":{"name":"oracle-url","key":"ORACLE_URL"}}},{"name":"CA_CERT_PATH","value":"/etc/pki/tls/certs/ca-bundle.crt"},{"name":"GIT_REPO_PATH","value":"/dags"},{"name":"AIRFLOW_DAG_ID","valueFrom":{"fieldRef":{"fieldPath":"metadata.annotations[''dag_id'']"}}},{"name":"AIRFLOW_RUN_ID","valueFrom":{"fieldRef":{"fieldPath":"metadata.annotations[''run_id'']"}}},{"name":"AIRFLOW_TASK_ID","valueFrom":{"fieldRef":{"fieldPath":"metadata.annotations[''task_id'']"}}},{"name":"AIRFLOW_DB_URL","valueFrom":{"secretKeyRef":{"name":"airflow-db","key":"connection"}}}],"image":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/knaudit:2023-09-04-34a8e3c","volumeMounts":[{"mountPath":"/dags","name":"dags-data"},{"mountPath":"/etc/pki/tls/certs/ca-bundle.crt","name":"ca-bundle-pem","readOnly":true,"subPath":"ca-bundle.pem"}]}]','airflow');

UPDATE chart_team_values SET "key" = 'dagRepo,omit' WHERE "key" = 'scheduler.extraContainers.[0].args.[0]';
UPDATE chart_team_values SET "key" = 'dagRepoBranch,omit' WHERE "key" = 'scheduler.extraContainers.[0].args.[1]';

DELETE FROM chart_team_values WHERE "key" IN (
    'scheduler.extraInitContainers.[0].args.[0]',
    'scheduler.extraInitContainers.[0].args.[1]',
    'workers.extraInitContainers.[0].args.[0]',
    'workers.extraInitContainers.[0].args.[1]',
    'webserver.extraContainers.[0].args.[0]',
    'webserver.extraContainers.[0].args.[1]'
)

-- +goose Down
DELETE FROM chart_global_values WHERE "key" = 'workers.extraInitContainers';
DELETE FROM chart_global_values WHERE "key" IN ('dags.gitSync.enabled','dags.gitSync.extraVolumeMounts','images.gitSync.repository','images.gitSync.tag');

INSERT INTO chart_global_values ("key","value","chart_type") VALUES 
    ('scheduler.extraContainers','[{"name":"git-sync","image":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/git-sync:2023-08-31-2f998de","resources":{"requests":{"cpu":"100m","memory":"128Mi","ephemeral-storage":"64Mi"}},"command":["/bin/sh","/git-sync.sh"],"args":["","","/dags","60"],"volumeMounts":[{"mountPath":"/dags","name":"dags-data"},{"mountPath":"/keys","name":"github-app-secret"}]}]','airflow'),
    ('scheduler.extraInitContainers','[{"name":"git-clone","image":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/git-sync:2023-08-31-2f998de","resources":{"requests":{"cpu":"100m","memory":"128Mi","ephemeral-storage":"64Mi"}},"command":["/bin/sh","/git-clone.sh"],"args":["","","/dags","60"],"volumeMounts":[{"mountPath":"/dags","name":"dags-data"},{"mountPath":"/keys","name":"github-app-secret"}]}]','airflow'),
    ('webserver.extraContainers','[{"name":"git-sync","image":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/git-sync:2023-08-31-2f998de","resources":{"requests":{"cpu":"100m","memory":"128Mi","ephemeral-storage":"64Mi"}},"command":["/bin/sh","/git-sync.sh"],"args":["","","/dags","60"],"volumeMounts":[{"mountPath":"/dags","name":"dags-data"},{"mountPath":"/keys","name":"github-app-secret"}]}]','airflow'),
    ('workers.extraInitContainers','[{"name":"git-clone","image":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/git-sync:2023-08-31-2f998de","command":["/bin/sh","/git-clone.sh"],"args":["","","/dags","60"],"volumeMounts":[{"mountPath":"/dags","name":"dags-data"},{"mountPath":"/keys","name":"github-app-secret"}]},{"name":"knaudit","env":[{"name":"NAMESPACE","valueFrom":{"fieldRef":{"fieldPath":"metadata.namespace"}}},{"name":"ORACLE_URL","valueFrom":{"secretKeyRef":{"name":"oracle-url","key":"ORACLE_URL"}}},{"name":"CA_CERT_PATH","value":"/etc/pki/tls/certs/ca-bundle.crt"},{"name":"GIT_REPO_PATH","value":"/dags"},{"name":"AIRFLOW_DAG_ID","valueFrom":{"fieldRef":{"fieldPath":"metadata.annotations[''dag_id'']"}}},{"name":"AIRFLOW_RUN_ID","valueFrom":{"fieldRef":{"fieldPath":"metadata.annotations[''run_id'']"}}},{"name":"AIRFLOW_TASK_ID","valueFrom":{"fieldRef":{"fieldPath":"metadata.annotations[''task_id'']"}}},{"name":"AIRFLOW_DB_URL","valueFrom":{"secretKeyRef":{"name":"airflow-db","key":"connection"}}}],"image":"europe-north1-docker.pkg.dev/knada-gcp/knada-north/knaudit:2023-09-04-34a8e3c","volumeMounts":[{"mountPath":"/dags","name":"dags-data"},{"mountPath":"/etc/pki/tls/certs/ca-bundle.crt","name":"ca-bundle-pem","readOnly":true,"subPath":"ca-bundle.pem"}]}]','airflow');

INSERT INTO chart_global_values ("key","value","chart_type") VALUES 
    ('scheduler.extraVolumes','[{"name":"dags-data","emptyDir":{}},{"name":"github-app-secret","secret":{"defaultMode":448,"secretName":"github-secret"}}]','airflow'),
    ('scheduler.extraVolumeMounts','[{"mountPath":"/dags","name":"dags-data"},{"mountPath":"/keys","name":"github-app-secret"}]','airflow'),
    ('workers.extraVolumes','[{"name":"dags-data","emptyDir":{}},{"name":"github-app-secret","secret":{"defaultMode":448,"secretName":"github-secret"}},{"name":"ca-bundle-pem","configMap":{"defaultMode":420,"name":"ca-bundle-pem"}}]','airflow'),
    ('workers.extraVolumeMounts','[{"mountPath":"/dags","name":"dags-data"},{"mountPath":"/keys","name":"github-app-secret"},{"mountPath":"/etc/pki/tls/certs/ca-bundle.crt","name":"ca-bundle-pem","readOnly":true,"subPath":"ca-bundle.pem"}]','airflow'),
    ('webserver.extraVolumes','[{"name":"airflow-auth","configMap":{"name":"airflow-auth-cm"}},{"name":"airflow-webserver","configMap":{"name":"airflow-webserver-cm"}},{"name":"dags-data","emptyDir":{}},{"name":"github-app-secret","secret":{"defaultMode":448,"secretName":"github-secret"}}]','airflow'),
    ('webserver.extraVolumeMounts','[{"mountPath":"/dags","name":"dags-data"},{"mountPath":"/keys","name":"github-app-secret"},{"mountPath":"/opt/airflow/auth.py","subPath":"auth.py","name":"airflow-auth"},{"mountPath":"/opt/airflow/webserver_config.py","subPath":"webserver_config.py","name":"airflow-webserver"}]','airflow');

INSERT INTO chart_team_values ('key','value','chart_type','team_id')
    (SELECT 'scheduler.extraInitContainers.[0].args.[0]', "value", "chart_type", "team_id" FROM chart_team_values WHERE "key" = 'dagRepo,omit'),
    (SELECT 'scheduler.extraInitContainers.[0].args.[1]', "value", "chart_type", "team_id" FROM chart_team_values WHERE "key" = 'dagRepoBranch,omit'),
    (SELECT 'scheduler.extraContainers.[0].args.[0]', "value", "chart_type", "team_id" FROM chart_team_values WHERE "key" = 'dagRepo,omit'),
    (SELECT 'scheduler.extraContainers.[0].args.[1]', "value", "chart_type", "team_id" FROM chart_team_values WHERE "key" = 'dagRepoBranch,omit'),
    (SELECT 'webserver.extraContainers.[0].args.[0]', "value", "chart_type", "team_id" FROM chart_team_values WHERE "key" = 'dagRepo,omit'),
    (SELECT 'webserver.extraContainers.[0].args.[1]', "value", "chart_type", "team_id" FROM chart_team_values WHERE "key" = 'dagRepoBranch,omit'),
    (SELECT 'workers.extraInitContainers.[0].args.[0]', "value", "chart_type", "team_id" FROM chart_team_values WHERE "key" = 'dagRepo,omit'),
    (SELECT 'workers.extraInitContainers.[0].args.[1]', "value", "chart_type", "team_id" FROM chart_team_values WHERE "key" = 'dagRepoBranch,omit');
