# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/api /app/api
ENV API_PORT=8080
EXPOSE 8080 9464
USER 65532:65532
ENTRYPOINT ["/app/api"]
