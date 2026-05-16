FROM golang:1.24 AS builder

WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -o server .

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/admin ./admin

CMD ["./server"]
