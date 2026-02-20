# Build stage
FROM golang:1.24.4-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .
RUN ls -la /app/migrations  
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server
RUN chmod +x /server

# Run stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /server .
COPY --from=builder /app/migrations ./migrations
RUN ls -la /app/migrations  

EXPOSE 8080

CMD ["./server"]
