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
WORKDIR /app
COPY --from=builder /src/knorten /app/knorten
COPY --from=builder /src/assets /app/assets
COPY --from=builder /src/templates /app/templates
CMD ["/app/knorten"]
