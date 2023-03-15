# Knelm

> Knada Helm Kubernetes jobs

## Deploy

Foreløpig bare manuell deploy.

1. Logg inn

        gcloud auth login --update-adc
2. Legg til Docker-config

        gcloud auth configure-docker europe-west1-docker.pkg.dev
3. Bygg image fra Knorten-root

        docker build -t europe-west1-docker.pkg.dev/knada-gcp/knada/knelm:$(git log -1 --pretty=%ad --date=format:%Y-%m-%d)-$(git log --pretty=format:'%h' -n 1) --platform linux/amd64 --file knelm/Dockerfile .
4. Push image

        docker push europe-west1-docker.pkg.dev/knada-gcp/knada/knelm:$(git log -1 --pretty=%ad --date=format:%Y-%m-%d)-$(git log --pretty=format:'%h' -n 1)
