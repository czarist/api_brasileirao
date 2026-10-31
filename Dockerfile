FROM golang:1.26.10-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM golang:1.26.10-bookworm AS test
WORKDIR /src

FROM alpine:3.21.3 AS runtime
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -g 65532 -S app \
    && adduser -D -u 65532 -G app -S app
COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker
COPY --from=build /out/migrate /app/migrate
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/api"]
