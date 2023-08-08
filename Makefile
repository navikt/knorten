.PHONY: env local local-offline generate-sql install-sqlc goose
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
	GOBIN=$(shell go env GOPATH)/bin
else
	GOBIN=$(shell go env GOBIN)
endif

-include .env

env:
	echo "AZURE_APP_CLIENT_ID=$(shell kubectl get secret --context=knada --namespace=knada-system knorten -o jsonpath='{.data.AZURE_APP_CLIENT_ID}' | base64 -d)" > .env
	echo "AZURE_APP_CLIENT_SECRET=$(shell kubectl get secret --context=knada --namespace=knada-system knorten -o jsonpath='{.data.AZURE_APP_CLIENT_SECRET}' | base64 -d)" >> .env
	echo "AZURE_APP_TENANT_ID=$(shell kubectl get secret --context=knada --namespace=knada-system knorten -o jsonpath='{.data.AZURE_APP_TENANT_ID}' | base64 -d)" >> .env

netpol:
	$(shell kubectl get --context=knada --namespace=knada-system configmap/airflow-network-policy -o json | jq -r '.data."default-egress-airflow-worker.yaml"' > .default-egress-airflow-worker.yaml)

local-online:
	go run -race . \
	  --admin-group=nada@nav.no \
	  --airflow-chart-version=1.10.0 \
	  --db-conn-string=postgres://postgres:postgres@localhost:5432/knorten \
	  --db-enc-key=jegersekstentegn \
	  --in-cluster=false \
	  --jupyter-chart-version=2.0.0 \
	  --oauth2-client-id=$(AZURE_APP_CLIENT_ID) \
	  --oauth2-client-secret=$(AZURE_APP_CLIENT_SECRET) \
	  --oauth2-tenant-id=$(AZURE_APP_TENANT_ID) \
	  --project=nada-dev-db2e \
	  --region=europe-west1 \
	  --session-key online-session
	  --zone=europe-west1-b \

local:
	HELM_REPOSITORY_CONFIG="./.helm-repositories.yaml" \
	go run -race . \
	  --admin-group=nada@nav.no \
	  --airflow-chart-version=1.10.0 \
	  --db-conn-string=postgres://postgres:postgres@localhost:5432/knorten \
	  --db-enc-key=jegersekstentegn \
	  --dry-run \
	  --in-cluster=false \
	  --jupyter-chart-version=2.0.0 \
	  --project=nada-dev-db2e \
	  --region=europe-west1 \
	  --session-key offline-session
	  --zone=europe-west1-b \

generate-sql:
	$(GOBIN)/sqlc generate

install-sqlc:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# make goose cmd=status
goose:
	goose -dir pkg/database/migrations/ postgres "user=postgres password=postgres dbname=knorten host=localhost sslmode=disable" $(cmd)

init:
	go run local/main.go

css:
	npx tailwindcss --postcss -i local/tailwind.css -o assets/css/main.css

css-watch:
	npx tailwindcss --postcss -i local/tailwind.css -o assets/css/main.css -w

test:
	go test -v ./... -count=1
