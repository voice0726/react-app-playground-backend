FROM golang:1.27.3-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /out/migrate /app/migrate
COPY migrations /app/migrations
EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/api"]
