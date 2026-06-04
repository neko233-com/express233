FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/neko233-com/express233/internal/version.Version=${VERSION}" \
    -o /out/express233-server ./cmd/express233-server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget
COPY --from=build /out/express233-server /usr/local/bin/
ENV EXPRESS233_DATA=/data
VOLUME /data
EXPOSE 23380
ENTRYPOINT ["express233-server"]
CMD ["-addr", ":23380", "-data", "/data"]
