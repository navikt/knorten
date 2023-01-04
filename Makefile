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
	echo "GCP_PROJECT=$(shell kubectl get secret --context=knada --namespace=knada-system knorten -o jsonpath='{.data.GCP_PROJECT}' | base64 -d)" >> .env
	echo "GCP_REGION=$(shell kubectl get secret --context=knada --namespace=knada-system knorten -o jsonpath='{.data.GCP_REGION}' | base64 -d)" >> .env
	echo "DB_ENC_KEY=$(shell kubectl get secret --context=knada --namespace=knada-system knorten -o jsonpath='{.data.DB_ENC_KEY}' | base64 -d)" >> .env

local:
	go run . \
	  --hostname=localhost \
	  --oauth2-client-id=$(AZURE_APP_CLIENT_ID) \
	  --oauth2-client-secret=$(AZURE_APP_CLIENT_SECRET) \
	  --oauth2-tenant-id=$(AZURE_APP_TENANT_ID) \
	  --project=$(GCP_PROJECT) \
	  --region=$(GCP_REGION) \
	  --db-enc-key=$(DB_ENC_KEY) \
	  --in-cluster=false \
	  --knelm-image=europe-west1-docker.pkg.dev/knada-gcp/knorten/knelm:v9 \
	  --db-conn-string=postgres://postgres:postgres@localhost:5432/knorten

local-offline:
	go run . \
	  --hostname=localhost \
	  --oauth2-client-id=$(AZURE_APP_CLIENT_ID) \
	  --oauth2-client-secret=$(AZURE_APP_CLIENT_SECRET) \
	  --oauth2-tenant-id=$(AZURE_APP_TENANT_ID) \
	  --project=$(GCP_PROJECT) \
	  --region=$(GCP_REGION) \
	  --db-enc-key=$(DB_ENC_KEY) \
	  --dry-run \
	  --in-cluster=false \
	  --knelm-image=europe-west1-docker.pkg.dev/knada-gcp/knorten/knelm:v9 \
	  --db-conn-string=postgres://postgres:postgres@localhost:5432/knorten

generate-sql:
	$(GOBIN)/sqlc generate

install-sqlc:
	go install github.com/kyleconroy/sqlc/cmd/sqlc@latest

# make goose cmd=status
goose:
	goose -dir pkg/database/migrations/ postgres "user=postgres password=postgres dbname=knorten host=localhost sslmode=disable" $(cmd)
