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
RUN apk add --no-cache ca-certificates wget

RUN addgroup -S app && adduser -S -G app -u 10001 app

COPY --from=build /out/mylab-api-go /usr/local/bin/mylab-api-go

ENV HTTP_ADDR=:8080
EXPOSE 8080

USER 10001:10001

ENTRYPOINT ["/usr/local/bin/mylab-api-go"]
