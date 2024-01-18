# Knorten

> KNADA sin port for bestilling av tjenester

## Produksjon

Oppsett for Azure AD er satt opp i [navikt/aad-iac](https://github.com/navikt/aad-iac/blob/master/prod/knorten.yaml).
Ellers blir Knorten satt opp gjennom [nais/knada-gcp](https://github.com/nais/knada-gcp/blob/main/knorten.tf).

## Utvikling

For å jobbe med Knorten lokalt trenger man å ha Postgres kjørende, og basen må prepouleres med litt data.
I tillegg benytter vi et oppsett med Tailwind og Designsystemet.

- `docker-compose up -d db` for å starte Postgres.
- Kjør `make init` etter du har kjørt opp Postgres for å populere databasen.
- Kjør `npm install` for å sette opp nødvendig rammeverk for Tailwind.

### Lokalt uten tredjeparter

Kjør `make local` for å kjøre Knorten uten kobling til noe annet enn Postgres.

### Lokalt med tredjeparter i eget cluster

Har man behov for å teste mot et cluster så kan man bruke `make local-online`, dette kobler deg opp til `nada-dev-db2e`, og et lokalt [Minikube](https://minikube.sigs.k8s.io/) cluster.

Sett opp minikube clusteret med: `minikube start --driver=qemu2 --kubernetes-version=v1.27.4`

Husk å skru på [gcp-auth](https://minikube.sigs.k8s.io/docs/handbook/addons/gcp-auth/) i Minikube.

PS: Hver gang du logger inn med `gcloud auth login --update-adc` må kjøre `minikube addons enable gcp-auth --refresh` for å oppdatere tokenet.

#### CRDer for Minikube

Man må legge til følgende CRDer når man kjører lokalt.

    kubectl apply --context minikube -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/main/config/crd/standard/gateway.networking.k8s.io_httproutes.yaml
    kubectl apply --context minikube -f https://raw.githubusercontent.com/GoogleCloudPlatform/gke-networking-recipes/main/gateway-api/config/servicepolicies/crd/standard/healthcheckpolicy.yaml

### Generering av CSS

Man kan generere CSS på en av to måter.

* `make css` kjører en engangsjobb som ser på template-filene og genererer CSS-klasser ut ifra det som er brukt
* `make css-watch` kjører samme jobb som over hver gang noe endres i template-filer

Idéelt sett kan man spinne opp `make css-watch` i en annen terminal samtidig som man kjører Knorten med f.eks. `make local-online`.

Filen som styrer hva som genereres finner du i `local/tailwind.css`, her kan du legge inn [Tailwind-regler](https://tailwindcss.com/docs/functions-and-directives#layer) som vanlig dersom nødvendig. Designsystem-regler blir generert uansett ved hjelp av `@import`-regelen i toppen av den filen.

### Designsystemet

Bruk av designsystemet til NAV krever litt kritisk tenking og manuelt arbeid.
Siden designsystemet i all hovedsak sikter på React-komponenter mens vi kun benytter CSS derfra, kan det være vanskelig å finne ut hvilke CSS-regler som faktisk gjelder for det vi ønsker å oppnå.
Likevel finnes det en lur (om ikke litt kjip) måte å finne CSS-regler på.

* Identifiser ønsket Designsystemkomponent i [Aksel](https://aksel.nav.no/komponenter)
* Scroll ned til eksempler (f.eks. [Button](https://aksel.nav.no/komponenter/core/button#ha8bb240d2c68))
* Høyreklikk på rendret eksempel og inspiser/inspect

Her vil du se hvilke klasser som ligger på komponenten du ønsker å lage.
Siden vi importerer `@navikt/ds-css` kan disse klassene brukes verbatim i koden vår.

### Postgres

Bruk Docker Compose:

    docker-compose up -d

eller Docker (med Adminer):

    docker run --name postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres
    docker run --link postgres:db -p 8081:8080 -d adminer

Hvis du allerede har en Postgres-instans kjørende kan du bare lage en ny database for Knorten:

    psql -h localhost -U postgres -c 'CREATE DATABASE knorten;'

## Tilgang til Postgres i prod

Trenger man tilgang til prod-databasen kan man gjøre dette med `gcloud` og `cloud_sql_proxy`.

```
CONNECTION_NAME=$(gcloud sql instances describe knorten-north --format="get(connectionName)" --project knada-gcp);
cloud_sql_proxy -enable_iam_login -instances=${CONNECTION_NAME}=tcp:5433
```

Trenger man å dumpe hele basen kan man bruke følgende kommandoer:
```
pg_dump -U knorten -h localhost -p5433 > knorten.sql
psql -U postgres -h localhost -p 5432 -d knorten -f knorten.sql
```

PS: Legg merke til at vi bruker port `5433` i kommandoene ovenfor, da man mest sannsynligvis har en Postgres-instans kjørende lokalt på `5432`.
