# syntax=docker/dockerfile:1.7

ARG IMAGE_REGISTRY=public.ecr.aws/docker
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn
FROM ${IMAGE_REGISTRY}/library/golang:1.26.5 AS build

ARG GOPROXY
ARG GOSUMDB
ENV GOPROXY=$GOPROXY
ENV GOSUMDB=$GOSUMDB

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    for attempt in 1 2 3; do \
        go mod download && break; \
        if [ "$attempt" -eq 3 ]; then exit 1; fi; \
        sleep $((attempt * 2)); \
    done

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/scheduler ./cmd/scheduler && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/seed-admin ./cmd/seed-admin

FROM scratch

WORKDIR /app
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build --chown=10001:10001 /out/api /app/api
COPY --from=build --chown=10001:10001 /out/worker /app/worker
COPY --from=build --chown=10001:10001 /out/scheduler /app/scheduler
COPY --from=build --chown=10001:10001 /out/migrate /app/migrate
COPY --from=build --chown=10001:10001 /out/seed-admin /app/seed-admin
COPY --chown=10001:10001 db/migrations/*.sql /app/migrations/

USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/app/api"]
