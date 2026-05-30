FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/gateway ./cmd/gateway \
    && go build -o /out/control-api ./cmd/control-api \
    && go build -o /out/configd ./cmd/configd \
    && go build -o /out/worker ./cmd/worker \
    && go build -o /out/migrate ./cmd/migrate

FROM alpine:3.22

WORKDIR /app
COPY --from=build /out/gateway /usr/local/bin/gateway
COPY --from=build /out/control-api /usr/local/bin/control-api
COPY --from=build /out/configd /usr/local/bin/configd
COPY --from=build /out/worker /usr/local/bin/worker
COPY --from=build /out/migrate /usr/local/bin/migrate
COPY configs/local.yaml /app/configs/local.yaml
COPY migrations /app/migrations

EXPOSE 9501 9502 9503 9504
CMD ["gateway", "-config", "configs/local.yaml"]
