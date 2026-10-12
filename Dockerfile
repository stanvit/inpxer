# syntax=docker/dockerfile:1

## Build
# Runs natively on the builder and cross-compiles, so multi-arch builds need no emulation
FROM --platform=$BUILDPLATFORM golang:1.27.0-bookworm AS build
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w -X main.version=$VERSION" -o /go/bin/inpxer

## fb2 converter, https://github.com/rupor-github/fb2cng
FROM --platform=$BUILDPLATFORM busybox:stable AS fbc-download
# The checksum pins the linux-amd64 asset, the only platform the image is built for
ADD --checksum=sha256:470d4c7b9b2f842b92c481b042563d704325e550b52b80b71cfd2481293b9191 \
    https://github.com/rupor-github/fb2cng/releases/download/v1.8.3/fbc-linux-amd64.zip /tmp/fbc.zip
RUN unzip /tmp/fbc.zip fbc -d /out

## Deploy
FROM gcr.io/distroless/static-debian13 AS plain

COPY --from=build /go/bin/inpxer /bin/inpxer

EXPOSE 8080/tcp
CMD ["inpxer", "serve"]

FROM plain AS fbc

COPY --from=fbc-download /out/fbc /bin/fbc

# Default target
FROM plain
