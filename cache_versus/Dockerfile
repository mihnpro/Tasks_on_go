FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /cache-comparison ./cmd/...

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /cache-comparison .
CMD ["/app/cache-comparison"]
