FROM node:24-alpine AS web-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
COPY internal/api/web/style.css /src/internal/api/web/style.css
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /src/internal/api/web/dist ./internal/api/web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/neko233-com/express233/internal/version.Version=${VERSION}" \
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
