FROM golang:1.25-alpine AS builder

LABEL org.opencontainers.image.source=https://github.com/coding-cave-dev/nimbul

WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/nimbul-api ./cmd/api/main.go

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 app
WORKDIR /
COPY --from=builder /out/nimbul-api /nimbul-api
USER app
EXPOSE 8080
ENTRYPOINT ["/nimbul-api"]
