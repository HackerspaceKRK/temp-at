FROM golang:alpine AS builder


# Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache git bash nodejs npm alpine-sdk gcc && mkdir -p /build/temp-at

WORKDIR /build/temp-at

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download -json

COPY . .

# Build frontend and embed
RUN go generate ./...

RUN mkdir -p /app && CGO_ENABLED=1 GOOS=${TARGETPLATFORM%%/*} GOARCH=${TARGETPLATFORM##*/} \
    go build -ldflags='-s -w -extldflags="-static"' -o /app/temp-at

FROM scratch AS bin-unix
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/temp-at /app/temp-at

LABEL org.opencontainers.image.description="A docker image for the temp-at microservice."
LABEL org.opencontainers.image.source="https://github.com/HackerspaceKRK/temp-at"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.title="temp-at"

ENTRYPOINT ["/app/temp-at"]
