FROM golang:1.22-alpine AS builder

WORKDIR /src
RUN apk add --no-cache build-base

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/composepulse ./main.go

FROM alpine:3.22

WORKDIR /app
RUN apk add --no-cache docker-cli docker-cli-compose ca-certificates \
    && mkdir -p /data

COPY --from=builder /out/composepulse /usr/local/bin/composepulse

EXPOSE 8087

ENTRYPOINT ["/usr/local/bin/composepulse"]
