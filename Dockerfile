FROM golang:1.25 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /review-api ./cmd/review-api

FROM debian:bookworm-slim
WORKDIR /app

COPY --from=builder /review-api /usr/local/bin/review-api
COPY migrations ./migrations

EXPOSE 8080
CMD ["review-api"]
