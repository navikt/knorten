# Knorten

> KNADA sin port for bestilling av tjenester

## Produksjon

Oppsett for Azure AD er satt opp i [navikt/aad-iac](https://github.com/navikt/aad-iac/blob/master/prod/knorten.yaml).
Ellers blir Knorten satt opp gjennom [nais/knada-gcp](https://github.com/nais/knada-gcp/blob/main/knorten.tf).

## Utvikling

For å jobbe med Knorten lokalt trenger man å ha Postgres kjørende, og basen må prepouleres med litt data.
Kjør `make init` etter du har kjørt opp Postgres.

### Lokalt uten tredjeparter

Kjør `make local` for å kjøre Knorten uten kobling til noe annet enn Postgres.

### Lokalt med tredjeparter

Har man behov for å teste mot et cluster så kan man bruke `make local-online`, bare husk å kjør `make env` for å hente ned variabler som trengs.

### Postgres

Bruk enten `docker-compose up -d`, eller hvis du allerede har en Postgres-instans kjørende kan du bruke `psql -h localhost -U postgres -c 'CREATE DATABASE knorten;' før du starter Knorten.

## Tilgang til Postgres i prod

Trenger man tilgang til prod-databasen kan man gjøre dette med `gcloud` og `cloud_sql_proxy`.

```
CONNECTION_NAME=$(gcloud sql instances describe knorten --format="get(connectionName)" --project knada-gcp);
cloud_sql_proxy -enable_iam_login -instances=${CONNECTION_NAME}=tcp:5433
```

Trenger man å dumpe hele basen kan man bruke følgende kommandoer:
```
pg_dump -U knorten -h localhost -p5433 > knorten.sql
psql -U postgres -h localhost -p 5432 -d knorten -f knorten.sql
```

PS: Legg merke til at vi bruker port `5433` i kommandoene overnfor, da man mest sannsynligvis har en Postgres-instans kjørende lokalt på `5432`.
