FROM golang:1.19-alpine as builder

ENV CGO_ENABLED=0

WORKDIR /src
COPY go.sum go.sum
COPY go.mod go.mod
RUN go mod download
COPY . .
RUN go build -o knelm ./knelm

FROM alpine:3

WORKDIR /app
COPY --from=builder /src/knelm/knelm /app/knelm
CMD ["/app/knelm"]
