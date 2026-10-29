# Build the Go API as a small static Linux binary.
FROM golang:1.27.3-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY config ./config
COPY internals ./internals
COPY models ./models
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/domaincheck ./cmd

# Keep the runtime image small and run the API without root privileges.
FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

WORKDIR /app
COPY --from=build /out/domaincheck /usr/local/bin/domaincheck
COPY --chown=10001:10001 dns.json /app/dns.json

ENV PORT=8000
EXPOSE 8000
USER 10001:10001

ENTRYPOINT ["/usr/local/bin/domaincheck"]
