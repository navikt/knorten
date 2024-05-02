# Knorten

> KNADA sin port for bestilling av tjenester

## Produksjon

Oppsett for Azure AD er satt opp i [navikt/aad-iac](https://github.com/navikt/aad-iac/blob/master/prod/knorten.yaml).
Ellers blir Knorten satt opp gjennom [nais/knada-gcp](https://github.com/nais/knada-gcp/blob/main/knorten.tf).

## Utvikling

For å jobbe med Knorten lokalt trenger man å ha Postgres kjørende, og basen må prepouleres med litt data.
I tillegg benytter vi et oppsett med Tailwind og Designsystemet.


### Lokalt uten Kubernetes og Helm

1. `docker-compose up -d db` for å starte Postgres.
2. `make init` etter du har kjørt opp Postgres for å populere databasen.
3. `npm install` for å sette opp nødvendig rammeverk for Tailwind.
4. `make local` for å starte Knorten uten kobling til noe annet enn Postgres.

### Lokalt med Kubernetes og Helm
NB: vi kopierer noen Secrets og ConfigMaps fra prod til minikube. Vi bruker `gke_knada-gcp_europe-north1_knada-gke` som default context, men du kan overkjøre denne ved å sette `PROD_KUBE_CONTEXT`-miljøvariabelen.

```bash
# Kjør opp alt
make run

# Velg en annen k8s context for å kopiere ut secrets og configmaps
KUBECTL_PROD_CTX=my-prod-name make run

# Sleng på ekstra argumenter til minikube start
MINIKUBE_START_ARGS="--cache-images=false" make run
```

Etter at applikaskonen er oppe og kjører kan du opprette en Airflow instans, etc. Azure AD er satt opp slik at du kan logge inn med NAV-ident. Men du må sette opp port-forward fra `airflow-webserver` til `localhost:8888` for å kunne logge inn.

```bash

For å fjerne **alt** igjen:

```bash
make clean
```

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
