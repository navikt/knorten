# Knorten

> KNADA sin port for bestilling av tjenester

## Produksjon

Oppsett for Azure AD er satt opp i [navikt/aad-iac](https://github.com/navikt/aad-iac/blob/master/prod/knorten.yaml).
Ellers blir Knorten satt opp gjennom [nais/knada-gcp](https://github.com/nais/knada-gcp/blob/main/knorten.tf).

## Utvikling

For å jobbe med Knorten lokalt trenger man å ha Postgres kjørende, og basen må prepouleres med litt data.
I tillegg benytter vi et oppsett med Tailwind og Designsystemet.
Kjør `make init` etter du har kjørt opp Postgres for å populere databasen.
Kjør `npm install` for å sette opp nødvendig rammeverk for Tailwind.

### Lokalt uten tredjeparter

Kjør `make local` for å kjøre Knorten uten kobling til noe annet enn Postgres.

### Lokalt med tredjeparter

Har man behov for å teste mot et cluster så kan man bruke `make local-online`, bare husk å kjør `make env` for å hente ned variabler som trengs.

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

Bruk enten `docker-compose up -d`, eller hvis du allerede har en Postgres-instans kjørende kan du bruke `psql -h localhost -U postgres -c 'CREATE DATABASE knorten;'` før du starter Knorten.

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
