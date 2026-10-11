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

## Deploy
FROM gcr.io/distroless/static-debian13

COPY --from=build /go/bin/inpxer /bin/inpxer

EXPOSE 8080/tcp
CMD ["inpxer", "serve"]
