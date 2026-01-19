# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./

ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/mylab-api-go ./cmd/mylab-api-go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget su-exec

RUN addgroup -S app && adduser -S -G app -u 10001 app

WORKDIR /app
RUN mkdir -p /app/storage/sessions && chown -R app:app /app

COPY --from=build /out/mylab-api-go /usr/local/bin/mylab-api-go
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

ENV HTTP_ADDR=:8080
ENV AUTH_SESSION_DRIVER=file
ENV AUTH_SESSION_FILES=/app/storage/sessions
EXPOSE 8080

USER root

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
