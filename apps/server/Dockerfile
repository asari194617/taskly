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

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV API_PORT=8080
EXPOSE 8080 9464
ENTRYPOINT ["/app/api"]
