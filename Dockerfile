FROM golang:1.19-alpine as builder
LABEL org.opencontainers.image.source https://github.com/nais/knorten

RUN apk add --no-cache git make
ENV GOOS=linux
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.sum go.sum
COPY go.mod go.mod
RUN go mod download
COPY . .
RUN make linux-build

FROM alpine:3

RUN apk update && apk add curl tar python3

# Install gcloud
RUN curl https://dl.google.com/dl/cloudsdk/release/google-cloud-sdk.tar.gz > /tmp/google-cloud-sdk.tar.gz
RUN mkdir -p /usr/local/gcloud \
    && tar -C /usr/local/gcloud -xvf /tmp/google-cloud-sdk.tar.gz \
    && /usr/local/gcloud/google-cloud-sdk/install.sh
ENV PATH $PATH:/usr/local/gcloud/google-cloud-sdk/bin

WORKDIR /app
COPY --from=builder /src/knorten /app/knorten
COPY --from=builder /src/assets /app/assets
COPY --from=builder /src/templates /app/templates
CMD ["/app/knorten"]
