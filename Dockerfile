FROM golang:1.22-alpine as builder

WORKDIR /src
COPY go.sum go.sum
COPY go.mod go.mod
RUN go mod download
COPY . .
RUN go build -o knorten .

FROM gcr.io/google.com/cloudsdktool/google-cloud-cli:alpine

RUN adduser -u 1001 knorten -D && \
    mkdir /home/knorten/.config && \
    chown -R knorten:knorten /home/knorten

RUN gcloud components install beta

WORKDIR /app
COPY --from=builder /src/knorten /app/knorten
COPY --from=builder /src/assets /app/assets
COPY --from=builder /src/templates /app/templates

CMD ["/app/knorten", "--config", "/app/config.yaml"]
