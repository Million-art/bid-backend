FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /api ./cmd/api

FROM alpine:latest
RUN apk add --no-cache ca-certificates curl tzdata
WORKDIR /app
COPY --from=builder /api .
EXPOSE 8080 5001

CMD ["./api"]
