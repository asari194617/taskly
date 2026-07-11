# syntax=docker/dockerfile:1

FROM golang:1.26.5 as build
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && go build -o /out/dmarc-processor ./...

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/dmarc-processor /app/dmarc-processor
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/app/dmarc-processor"]
