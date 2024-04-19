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

WORKDIR /home/knorten
COPY --from=builder /src/knorten /home/knorten/knorten
COPY --from=builder /src/assets /home/knorten/assets
COPY --from=builder /src/templates /home/knorten/templates

CMD ["/home/knorten/knorten", "--config", "/home/knorten/config.yaml"]
