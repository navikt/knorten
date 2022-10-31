.PHONY: test integration-test local-with-auth local linux-build docker-build docker-push run-postgres-test stop-postgres-test install-sqlc 
DATE = $(shell date "+%Y-%m-%d")
LAST_COMMIT = $(shell git --no-pager log -1 --pretty=%h)
VERSION ?= $(DATE)-$(LAST_COMMIT)
LDFLAGS := -X github.com/navikt/nada-backend/backend/version.Revision=$(shell git rev-parse --short HEAD) -X github.com/navikt/nada-backend/backend/version.Version=$(VERSION)
APP = nada-backend
SQLC_VERSION ?= "v1.15.0"
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
	GOBIN=$(shell go env GOPATH)/bin
else
	GOBIN=$(shell go env GOBIN)
endif

-include .env

local:
	go run ./cmd/nada-backend \
	  --bind-address=127.0.0.1:8080 \
	  --hostname=localhost \
	  --mock-auth \
	  --skip-metadata-sync \
	  --log-level=debug

generate-sql:
	cd pkg && $(GOBIN)/sqlc generate

install-sqlc:
	go install github.com/kyleconroy/sqlc/cmd/sqlc@$(SQLC_VERSION)
