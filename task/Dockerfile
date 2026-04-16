# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /online-store ./cmd

# Run stage
FROM alpine:latest
RUN apk update && apk add --no-cache ca-certificates
WORKDIR /root/
COPY --from=builder /online-store .
EXPOSE 8080
CMD ["./online-store"]