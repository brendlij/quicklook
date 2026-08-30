# quicklook

**One container. One page. See what your server is doing.**

quicklook is an extremely lightweight, self-hosted Linux server dashboard. It reads host metrics directly from `/proc` and `/sys`, talks to the Docker Engine over its Unix socket, and serves a polished live interface from one Go binary. There is no database, monitoring stack, frontend runtime, or configuration ceremony.

> Screenshot placeholder — run quicklook and capture the Overview page at `http://localhost:7373`.

## Features

- Live CPU, memory, load, uptime, temperature, network, disk I/O, and filesystem usage
- Five minutes of rolling in-memory history by default
- Optional read-only Docker inventory with per-container CPU, memory, network, health, ports, and restart information
- Server-Sent Events for efficient live updates from one central collector
- Responsive, keyboard-accessible dark interface with no frontend dependencies
- One static Go binary in a minimal `scratch` container
- Graceful operation when Docker, temperature sensors, or individual metrics are unavailable

## Quick start

```sh
docker compose up -d
```

Open [http://localhost:7373](http://localhost:7373).

The included Compose file mounts the host metric filesystems and Docker socket read-only. It does not require privileged mode.

To run without Docker visibility, remove the Docker socket volume and set `QUICKLOOK_DOCKER=false`.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `QUICKLOOK_PORT` | `7373` | HTTP listen port |
| `QUICKLOOK_HOST_PROC` | `/proc` | Host procfs path |
| `QUICKLOOK_HOST_SYS` | `/sys` | Host sysfs path |
| `QUICKLOOK_HOST_ROOT` | `/` | Host root used for filesystem usage and OS metadata |
| `QUICKLOOK_HISTORY_SECONDS` | `300` | Rolling history duration |
| `QUICKLOOK_INTERVAL` | `2s` | Collection interval (minimum `500ms`) |
| `QUICKLOOK_DOCKER` | `true` | Enable automatic Docker monitoring |
| `QUICKLOOK_DOCKER_SOCKET` | `/var/run/docker.sock` | Docker Engine Unix socket |

History is deliberately ephemeral and is reset whenever quicklook restarts.

## Architecture

```text
/proc + /sys + host root       Docker Engine API
           \                       /
            central metrics sampler
                      |
             concurrency-safe state
                 /            \
            REST API           SSE
                 \            /
                  embedded UI
```

All host reads happen in the central sampler. Connecting another browser does not create another `/proc` polling loop. The UI files are embedded with `go:embed`; the production artifact is a single executable.

## Security

quicklook does not expose stop, restart, delete, exec, or other container controls. Its Docker API usage is read-only at the application level.

**Important:** access to `/var/run/docker.sock` is effectively root-equivalent access to the host Docker daemon. Marking the socket mount `:ro` only makes the socket filesystem entry read-only; it does not restrict which Docker API operations a process could request. Only deploy trusted images, limit dashboard network exposure, and place authentication or an access-controlled reverse proxy in front of quicklook when it is reachable beyond a trusted network.

quicklook itself has no authentication in v1 and is intended for trusted networks.

## Development

Go 1.22 or newer is required.

```sh
go test ./...
go run ./cmd/quicklook
```

On Linux, a local run uses `/proc`, `/sys`, `/`, and the Docker socket directly. The project still compiles on other operating systems so parsing and web development are convenient, but host collection is Linux-oriented.

Build the production container with:

```sh
docker build --build-arg VERSION=dev -t quicklook .
```

### Published images

The included GitHub Actions workflow validates the multi-platform image on pull requests and publishes `linux/amd64` and `linux/arm64` images to GitHub Container Registry on pushes to `main` and tags matching `v*`.

```sh
docker pull ghcr.io/brendlij/quicklook:latest
```

A tag such as `v1.2.3` also publishes `1.2.3` and `1.2` image tags. The workflow authenticates with the repository-provided `GITHUB_TOKEN`; no registry secret is required.

The first GHCR package is private by default. To allow anonymous `docker pull`, open the package settings on GitHub and change its visibility to public.

## API

All API responses are JSON, except the SSE stream.

| Endpoint | Purpose |
| --- | --- |
| `GET /healthz` | Liveness check |
| `GET /api/v1/status` | Complete current snapshot and history |
| `GET /api/v1/cpu` | Current processor state |
| `GET /api/v1/memory` | Current memory and swap state |
| `GET /api/v1/storage` | Filesystems and disk throughput |
| `GET /api/v1/network` | Interfaces and network throughput |
| `GET /api/v1/containers` | Docker availability and containers |
| `GET /api/v1/history` | Rolling samples |
| `GET /api/v1/events` | Live `snapshot` events over SSE |

The service intentionally has no mutating endpoints.
