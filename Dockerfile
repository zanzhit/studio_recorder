FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o migrator cmd/migrator/main.go
RUN go build -o recorder cmd/recorder/main.go

FROM alpine:latest

RUN apk update && apk --no-cache add \
    ca-certificates \
    postgresql-client \
    gstreamer \
    gst-plugins-base \
    gst-plugins-good \
    gst-plugins-bad \
    gst-plugins-ugly \
    gstreamer-tools \
    tzdata && \
    cp /usr/share/zoneinfo/Europe/Moscow /etc/localtime && \
    echo "Europe/Moscow" > /etc/timezone && \
    apk del tzdata

WORKDIR /root/

COPY --from=builder /app/migrator .
COPY --from=builder /app/recorder .
COPY ./config /root/config
COPY ./migrations /root/migrations
COPY wait-for-postgres.sh /root/

RUN chmod +x /root/wait-for-postgres.sh

CMD ["./recorder"]
