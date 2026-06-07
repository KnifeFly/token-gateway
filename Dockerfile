FROM node:22-alpine AS web-build

WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY web ./web
RUN corepack enable && pnpm install --frozen-lockfile
RUN pnpm build

FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/gateway ./cmd/gateway \
    && go build -o /out/control-api ./cmd/control-api \
    && go build -o /out/configd ./cmd/configd \
    && go build -o /out/console ./cmd/console \
    && go build -o /out/worker ./cmd/worker \
    && go build -o /out/migrate ./cmd/migrate

FROM alpine:3.22

WORKDIR /app
COPY --from=build /out/gateway /usr/local/bin/gateway
COPY --from=build /out/control-api /usr/local/bin/control-api
COPY --from=build /out/configd /usr/local/bin/configd
COPY --from=build /out/console /usr/local/bin/console
COPY --from=build /out/worker /usr/local/bin/worker
COPY --from=build /out/migrate /usr/local/bin/migrate
COPY --from=web-build /src/web/apps/portal/dist /app/web/apps/portal/dist
COPY --from=web-build /src/web/apps/admin/dist /app/web/apps/admin/dist
COPY configs/local.yaml /app/configs/local.yaml
COPY migrations /app/migrations

EXPOSE 9501 9502 9503 9504 9505
CMD ["gateway", "-config", "configs/local.yaml"]
