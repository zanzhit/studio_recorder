FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o migrator cmd/migrator/main.go

RUN go build -o recorder cmd/recorder/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates postgresql-client

WORKDIR /root/

COPY --from=builder /app/migrator .
COPY --from=builder /app/recorder .
COPY ./config /root/config
COPY ./migrations /root/migrations
COPY wait-for-postgres.sh /root/

RUN chmod +x /root/wait-for-postgres.sh

CMD ["./recorder"]