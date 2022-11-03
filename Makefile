.PHONY: local install-sqlc linux-build
DATE = $(shell date "+%Y-%m-%d")
LAST_COMMIT = $(shell git --no-pager log -1 --pretty=%h)
VERSION ?= $(DATE)-$(LAST_COMMIT)
APP = knorten
SQLC_VERSION ?= "v1.15.0"
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
	GOBIN=$(shell go env GOPATH)/bin
else
	GOBIN=$(shell go env GOBIN)
endif

-include .env

env:
	echo "AZURE_APP_CLIENT_ID=$(shell kubectl get secret --context=knada --namespace=knada-systems azureadapp -o jsonpath='{.data.AZURE_APP_CLIENT_ID}' | base64 -d)" > .env
	echo "AZURE_APP_CLIENT_SECRET=$(shell kubectl get secret --context=knada --namespace=knada-systems azureadapp -o jsonpath='{.data.AZURE_APP_CLIENT_SECRET}' | base64 -d)" >> .env
	echo "AZURE_APP_TENANT_ID=$(shell kubectl get secret --context=knada --namespace=knada-systems azureadapp -o jsonpath='{.data.AZURE_APP_TENANT_ID}' | base64 -d)" >> .env

local:
	go run . \
	  --hostname=localhost \
	  --oauth2-client-id=$(AZURE_APP_CLIENT_ID) \
	  --oauth2-client-secret=$(AZURE_APP_CLIENT_SECRET) \
	  --oauth2-tenant-id=$(AZURE_APP_TENANT_ID) \
	  --db-conn-string=postgres://postgres:postgres@localhost:5432/knorten

generate-sql:
	cd pkg && $(GOBIN)/sqlc generate

install-sqlc:
	go install github.com/kyleconroy/sqlc/cmd/sqlc@$(SQLC_VERSION)

linux-build:
	go build -a -installsuffix cgo -o $(APP) .
