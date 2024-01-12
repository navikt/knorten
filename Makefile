GOPATH := $(shell go env GOPATH)
GOBIN  ?= $(GOPATH)/bin # Default GOBIN if not set

# A template function for installing binaries
define install-binary
	 @if ! command -v $(1) &> /dev/null; then \
		  echo "$(1) not found, installing..."; \
		  go install $(2); \
	 fi
endef

GOOSE         ?= $(shell command -v goose || echo "$(GOBIN)/goose")
GOOSE_VERSION := v3.17.0
SQLC          ?= $(shell command -v sqlc || echo "$(GOBIN)/sqlc")
SQLC_VERSION  := v1.25.0

$(GOOSE):
	$(call install-binary,goose,github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION))

$(SQLC):
	$(call install-binary,sqlc,github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION))

-include .env

env:
	echo "AZURE_APP_CLIENT_ID=$(shell kubectl get secret --context=knada --namespace=knada-system knorten -o jsonpath='{.data.AZURE_APP_CLIENT_ID}' | base64 -d)" > .env
	echo "AZURE_APP_CLIENT_SECRET=$(shell kubectl get secret --context=knada --namespace=knada-system knorten -o jsonpath='{.data.AZURE_APP_CLIENT_SECRET}' | base64 -d)" >> .env
	echo "AZURE_APP_TENANT_ID=$(shell kubectl get secret --context=knada --namespace=knada-system knorten -o jsonpath='{.data.AZURE_APP_TENANT_ID}' | base64 -d)" >> .env
.PHONY: env

netpol:
	$(shell kubectl get --context=knada --namespace=knada-system configmap/airflow-network-policy -o json | jq -r '.data."default-egress-airflow-worker.yaml"' > .default-egress-airflow-worker.yaml)
.PHONY: netpol

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
	  --region=europe-north1 \
	  --session-key online-session
	  --zone=europe-north1-b
.PHONY: local-online

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
	  --region=europe-north1 \
	  --session-key offline-session
	  --zone=europe-north1-b
.PHONY: local

generate-sql: $(SQLC)
	$(SQLC) generate
.PHONY: generate-sql

# make goose cmd=status
goose: $(GOOSE)
	$(GOOSE) -dir pkg/database/migrations/ postgres "user=postgres password=postgres dbname=knorten host=localhost sslmode=disable" $(cmd)
.PHONY: goose

init:
	go run local/main.go
.PHONY: init

css:
	npx tailwindcss --postcss -i local/tailwind.css -o assets/css/main.css
.PHONY: css

css-watch:
	npx tailwindcss --postcss -i local/tailwind.css -o assets/css/main.css -w
.PHONY: css-watch

test:
	HELM_REPOSITORY_CONFIG="./.helm-repositories.yaml" go test -v ./... -count=1
.PHONY: test
