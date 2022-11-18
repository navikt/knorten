FROM golang:1.19-alpine as builder

ENV CGO_ENABLED=0

WORKDIR /src
COPY go.sum go.sum
COPY go.mod go.mod
RUN go mod download
COPY . .
RUN go build -o knorten .

FROM gcr.io/google.com/cloudsdktool/google-cloud-cli:alpine

WORKDIR /app
COPY --from=builder /src/knorten /app/knorten
COPY --from=builder /src/assets /app/assets
COPY --from=builder /src/templates /app/templates
CMD ["/app/knorten"]
