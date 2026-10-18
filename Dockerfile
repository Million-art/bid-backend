FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /admin-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /admin-migrate ./cmd/migrate

FROM alpine:latest
RUN apk add --no-cache ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /admin-api .
COPY --from=builder /admin-migrate .
EXPOSE 8080 5001

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://127.0.0.1:8080/health || exit 1

CMD ["./admin-api"]
