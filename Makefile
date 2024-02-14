GOPATH := $(shell go env GOPATH)
GOBIN  ?= $(GOPATH)/bin # Default GOBIN if not set

GCP_PROJECT_ID ?= knada-gcp

# A template function for installing binaries
define install-binary
	 @if ! command -v $(1) &> /dev/null; then \
		  echo "$(1) not found, installing..."; \
		  go install $(2); \
	 fi
endef

GOOSE                ?= $(shell command -v goose || echo "$(GOBIN)/goose")
GOOSE_VERSION        := v3.17.0
SQLC                 ?= $(shell command -v sqlc || echo "$(GOBIN)/sqlc")
SQLC_VERSION         := v1.25.0
GOLANGCILINT         ?= $(shell command -v golangci-lint || echo "$(GOBIN)/golangci-lint")
GOLANGCILINT_VERSION := v1.55.2

$(GOOSE):
	$(call install-binary,goose,github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION))

$(SQLC):
	$(call install-binary,sqlc,github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION))

$(GOLANGCILINT):
	$(call install-binary,golangci-lint,github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCILINT_VERSION))

env:
	echo "KNORTEN_OAUTH_CLIENT_ID=$$(gcloud secrets versions access latest --project=$(GCP_PROJECT_ID) --secret=knorten-oauth-client-id)" > .env
	echo "KNORTEN_OAUTH_CLIENT_SECRET=$$(gcloud secrets versions access latest --project=$(GCP_PROJECT_ID) --secret=knorten-oauth-client-secret)" >> .env
	echo "KNORTEN_OAUTH_TENANT_ID=$$(gcloud secrets versions access latest --project=$(GCP_PROJECT_ID) --secret=knorten-azure-tenant-id)" >> .env
.PHONY: env

netpol:
	$(shell kubectl get --context=knada --namespace=knada-system configmap/airflow-network-policy -o json | jq -r '.data."default-egress-airflow-worker.yaml"' > .default-egress-airflow-worker.yaml)
.PHONY: netpol

local-online:
	@echo "Sourcing environment variables..."
	set -a && source ./.env && set +a && \
		HELM_REPOSITORY_CONFIG="./.helm-repositories.yaml" go run -race . --config=config-local-online.yaml
.PHONY: local-online

local:
	@echo "Sourcing environment variables..."
	set -a && source ./.env && set +a && \
		HELM_REPOSITORY_CONFIG="./.helm-repositories.yaml" go run -race . --config=config-local.yaml
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

lint: $(GOLANGCILINT)
	$(GOLANGCILINT) run
.PHONY: lint

update-configmap:
	kubectl create configmap knorten-config \
		--from-file=config.yaml=config-prod.yaml --dry-run=client -o yaml \
			> k8s/configmap.yaml

gauth:
	@gcloud auth application-default print-access-token >/dev/null 2>&1 \
		&& gcloud auth list --filter=status:ACTIVE --format="value(account)" | grep -q "@nav.no" \
			&& echo "GCP ADC login not required." || gcloud auth login --update-adc
.PHONY: gauth

KUBERNETES_VERSION ?= v1.28.3
minikube: | gauth
	@minikube status >/dev/null 2>&1 && echo "Minikube is already running." || \
		minikube start --addons=gcp-auth,volumesnapshots --kubernetes-version=$(KUBERNETES_VERSION)
	@minikube addons enable gcp-auth --refresh
.PHONY: minikube

minikube-destroy:
	@minikube delete
.PHONY: minikube-destroy

deps: | minikube
	HELM_REPOSITORY_CONFIG="./.helm-repositories.yaml" helm upgrade --install \
		cnpg --namespace cnpg-system --create-namespace cnpg/cloudnative-pg
	docker-compose up -d db
.PHONY: deps

# These are our main targets

check: | lint test
.PHONY: check

run: | deps env init local-online
.PHONY: run

clean: | minikube-destroy
	@rm .env || echo "No .env file found."
	@docker-compose down --volumes
	@gcloud auth revoke || echo "No active account found."
	@gcloud auth application-default revoke --quiet || echo "No active token found."
.PHONY: clean
