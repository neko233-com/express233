FROM node:24-bookworm-slim AS console
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./frontend/
RUN npm --prefix frontend ci
COPY frontend/ ./frontend/
COPY scripts/safe-deploy.sh ./scripts/
RUN npm --prefix frontend run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=console /src/internal/api/web/dist ./internal/api/web/dist/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/neko233-com/express233/internal/version.Version=${VERSION#v}" \
    -o /out/express233-server ./cmd/express233-server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget
COPY --from=build /out/express233-server /usr/local/bin/
ENV EXPRESS233_DATA=/data \
    EXPRESS233_ADDR=0.0.0.0:23380
VOLUME /data
EXPOSE 23380
ENTRYPOINT ["express233-server"]
CMD ["-addr", "0.0.0.0:23380", "-data", "/data"]
