# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/gobin ./cmd/gobin

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /site

COPY --from=builder /out/gobin /usr/local/bin/gobin
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

ENTRYPOINT ["docker-entrypoint.sh"]

EXPOSE 8080

CMD ["gobin", "serve", "--port", "8080", "--watch=true", "--live-reload=false"]
