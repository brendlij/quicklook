# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /quicklook ./cmd/quicklook

FROM scratch
COPY --from=build /quicklook /quicklook
EXPOSE 7373
ENTRYPOINT ["/quicklook"]
