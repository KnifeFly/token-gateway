FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/gateway ./cmd/gateway

FROM alpine:3.22

WORKDIR /app
COPY --from=build /out/gateway /usr/local/bin/token-gateway
COPY configs/local.yaml /app/configs/local.yaml
COPY migrations /app/migrations

EXPOSE 9501
ENTRYPOINT ["token-gateway", "-config", "configs/local.yaml"]
