FROM golang:1.23.3-alpine AS builder
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN go build -o /out/sleeper-archive ./cmd/sleeper-archive

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /
COPY --from=builder /out/sleeper-archive /sleeper-archive
ENTRYPOINT ["/sleeper-archive"]
