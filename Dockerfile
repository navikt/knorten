FROM golang:1.25-alpine as builder

WORKDIR /src
COPY go.sum go.sum
COPY go.mod go.mod
RUN go mod download
COPY . .
RUN go build -o knorten .

RUN adduser -u 1001 knorten -D && \
    mkdir -p /home/knorten/.config/helm && \
    chown -R knorten:knorten /home/knorten

FROM gcr.io/distroless/static-debian11

COPY --chown=knorten:knorten --from=builder /etc/passwd /etc/passwd
COPY --chown=knorten:knorten --from=builder /home/knorten /home/knorten
COPY --chown=knorten:knorten --from=builder /home/knorten/.config/helm /home/knorten/.config/helm
COPY --chown=knorten:knorten --from=builder /src/knorten /home/knorten/knorten
COPY --chown=knorten:knorten --from=builder /src/assets /home/knorten/assets
COPY --chown=knorten:knorten --from=builder /src/templates /home/knorten/templates

WORKDIR /home/knorten
CMD ["/home/knorten/knorten", "--config", "/home/knorten/config.yaml"]
