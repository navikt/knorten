# Process for database restore

1. Select a cluster and team namespace
2. Find a Cluster and the latest Backup definition within the cluster and namespace
3. Scale down Airflow webserver and scheduler
4. Create a new Cluster with a restore field present
5. Wait for restore to complete
6. Switch to new database

## Run the database restore preparation script

This is a read-only operation that will fetch the required data from a team namespace.

```bash
# Development
./prepare_restore.sh gke_knada-dev_europe-north1_knada-gke-dev [namespace]

# Production
./prepare_restore.sh gke_knada-gcp_europe-north1_knada-gke [namespace]
```

## Apply the restore operation

```bash
# Development
./apply_restore.sh gke_knada-dev_europe-north1_knada-gke-dev [namespace]

# Production
./apply_restore.sh gke_knada-gcp_europe-north1_knada-gke [namespace]
```

Verify the status of the restored database

### Switch the database

```bash
# Development
./switch_database.sh gke_knada-dev_europe-north1_knada-gke-dev [namespace]

# Production
./switch_database.sh gke_knada-gcp_europe-north1_knada-gke [namespace]
```
