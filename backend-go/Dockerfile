FROM golang:1.25-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /chaosduck-server ./cmd/server

# Runtime
FROM alpine:3.21

RUN apk --no-cache add ca-certificates

COPY --from=builder /chaosduck-server /chaosduck-server
COPY internal/db/migrations /migrations

EXPOSE 8080

CMD ["/chaosduck-server"]
