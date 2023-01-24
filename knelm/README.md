# Knelm

> Knada Helm Kubernetes jobs

## Deploy

Foreløpig bare manuell deploy.
Vi bruker løpenummer på release.

1. Logg inn med `gcloud auth login --update-adc`
2. Legg til Docker-config `gcloud auth configure-docker europe-west1-docker.pkg.dev`
3. Bygg image `docker build -t europe-west1-docker.pkg.dev/knada-gcp/knorten/knelm:v? --platform linux/amd64 --file knelm/Dockerfile .`
   1. Bygges fra Knorten-root.
4. Push image `docker push europe-west1-docker.pkg.dev/knada-gcp/knorten/knelm:v?`
