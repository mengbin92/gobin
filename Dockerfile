# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath \
    -ldflags="-s -w -X 'github.com/mengbin92/gobin/cmd/gobin/commands.Version=${VERSION}' -X 'github.com/mengbin92/gobin/cmd/gobin/commands.Commit=${COMMIT}' -X 'github.com/mengbin92/gobin/cmd/gobin/commands.BuildDate=${BUILD_DATE}'" \
    -o /out/gobin ./cmd/gobin

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /site

COPY --from=builder /out/gobin /usr/local/bin/gobin
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

ENTRYPOINT ["docker-entrypoint.sh"]

EXPOSE 8080

CMD ["gobin", "serve", "--port", "8080", "--watch=true", "--live-reload=false"]
