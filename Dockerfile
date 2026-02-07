FROM golang:1.25.6-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o bin/server cmd/main.go


FROM alpine:3.20 AS app

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/bin/server .

USER nobody:nobody
ENTRYPOINT ["./server"]
