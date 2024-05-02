# Process for database migration

1. Scale down Airflow webserver and scheduler to 0
2. Create a cloudnative-pg cluster by initialising from CloudSQL
3. Which will create a {team_name}-app secret with a connection URI
4. Update airflow-db secret to point to new cluster, by picking out uri from pgnative db
5. Scale up Airflow webserver and scheduler to 2

## Run the database migration preparation script

This is a read-only operation that will fetch data from all team namespaces, more specifically the existing database 
connection 
data, parse it, and generate 
manifests for creating a new postgres cluster that will be populated by duplicating data from CloudSQL. 

```bash
# Development
./prepare_migration.sh gke_knada-dev_europe-north1_knada-gke-dev

# Production
./prepare_migration.sh gke_knada-gcp_europe-north1_knada-gke
```

## Apply the migration for one team at a time

You can see all namespaces by running `tree migration-backup`. 

```bash
# Development
./apply_migration.sh gke_knada-dev_europe-north1_knada-gke-dev [namespace]

# Production
./apply_migration.sh gke_knada-gcp_europe-north1_knada-gke [namespace]
```

Verify the status of the migrated database

### Rollback the database

```bash
# Development
./rollback_migration.sh gke_knada-dev_europe-north1_knada-gke-dev [namespace]

# Production
./rollback_migration.sh gke_knada-gcp_europe-north1_knada-gke [namespace]
```

### Switch the database

```bash
# Development
./switch_database.sh gke_knada-dev_europe-north1_knada-gke-dev [namespace]

# Production
./switch_database.sh gke_knada-gcp_europe-north1_knada-gke [namespace]
```
