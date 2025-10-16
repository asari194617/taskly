# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /out/api ./

# Extract CA certificates (for outbound HTTPS if needed)
FROM alpine:3.20 AS certs
RUN apk add --no-cache ca-certificates

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata bash curl grep sed findutils mariadb-client
COPY --from=build /out/api /app/api
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV API_PORT=8080
EXPOSE 8080 9464

# Backup script and cron
COPY ./scripts/backup.sh /usr/local/bin/backup.sh
RUN chmod +x /usr/local/bin/backup.sh && echo "TZ=Asia/Tokyo" > /etc/crontabs/root \
	&& echo "# Daily MySQL backup at 03:00" >> /etc/crontabs/root \
	&& echo "0 3 * * * /usr/local/bin/backup.sh >> /var/log/backup.log 2>&1" >> /etc/crontabs/root

# Entrypoint runs API and crond side-by-side
COPY ./scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
