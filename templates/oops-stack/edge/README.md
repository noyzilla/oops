# Edge Layer — Reverse Proxy & Auto-SSL Guide

The Edge layer terminates public HTTP/HTTPS traffic, handles automatic SSL certificates (Let's Encrypt / ZeroSSL), and routes incoming requests to internal container workloads.

This template provides 3 production-ready options. Choose the proxy technology that best matches your workflow:

---

## Reverse Proxy Comparison Matrix

| Feature | Caddy Proxy (`caddy-docker-proxy`) | Traefik (`v3.x`) | Nginx-Proxy (+ `acme-companion`) |
| :--- | :--- | :--- | :--- |
| **Default in Oops** | **Yes (Recommended)** | Alternative | Alternative |
| **Config Style** | **Docker Labels** (Minimal 1-2 lines) | **Docker Labels** (Router/Service/Middleware) | **Environment Variables** (`VIRTUAL_HOST`) |
| **Zero-Config File** | **100%** (No config files needed) | ~90% (CLI flags in compose) | ~90% (Requires 2 containers) |
| **Auto-SSL Engine** | Native Caddy (Fastest & most reliable) | Native ACME resolver | Sidecar `acme-companion` container |
| **HTTP/3 (QUIC)** | Supported out of the box | Supported via entrypoint flag | Requires custom Nginx build |
| **Memory Footprint** | ~30 – 50 MB | ~40 – 70 MB | ~60 MB (2 containers combined) |
| **Web Dashboard** | None (Headless) | Built-in Web UI Dashboard | None |
| **Configuration Complexity** | Very Low | Moderate | Low |

---

## Option 1: Caddy Proxy (Default — Recommended)

### Overview
Uses `lucaslorentz/caddy-docker-proxy` to dynamically generate Caddy configuration on the fly from Docker container labels.

### Pros
- **Zero Config Files**: No static `Caddyfile` or proxy files required.
- **Automatic HTTPS & HTTP/3**: Handles certificate requests, renewals, and HTTP-to-HTTPS redirects automatically.
- **Concise Label Syntax**: Only 2 label lines needed per container.

### Cons & Trade-offs
- No graphical dashboard.

### Setup & Usage
```bash
# Start default Caddy edge proxy
oops up edge
```

### Application Container Labels (`apps/docker-compose.yml`)
```yaml
services:
  my-app:
    image: my-app:latest
    labels:
      # Caddy Auto-SSL Reverse Proxy
      caddy: "app.example.com"
      caddy.reverse_proxy: "{{upstreams 80}}"
    networks:
      - oops_network
```

---

## Option 2: Traefik v3 (Cloud-Native Proxy)

### Overview
Uses `traefik:v3.1` as a cloud-native dynamic reverse proxy with built-in metrics, web dashboard, and flexible routing middleware.

### Pros
- **Web UI Dashboard**: Visual inspection of active routers, services, and middlewares.
- **Advanced Routing**: Path prefixes, strip prefixes, canary deployments, and rate limiting middlewares.
- **Native Let's Encrypt**: Manages SSL certificates natively without extra sidecars.

### Cons & Trade-offs
- Verbose label syntax (requires configuring routers, entrypoints, and certificate resolvers separately).

### Setup & Usage
```bash
# Start Traefik edge proxy
docker compose -f edge/docker-compose.traefik.yml up -d
```

### Application Container Labels (`apps/docker-compose.yml`)
```yaml
services:
  my-app:
    image: my-app:latest
    labels:
      traefik.enable: "true"
      traefik.http.routers.myapp.rule: "Host(`app.example.com`)"
      traefik.http.routers.myapp.entrypoints: "websecure"
      traefik.http.routers.myapp.tls.certresolver: "letsencrypt"
    networks:
      - oops_network
```

---

## Option 3: Nginx Proxy + ACME Companion (Classic Nginx)

### Overview
Uses the classic `nginxproxy/nginx-proxy` paired with `nginxproxy/acme-companion` for teams with strict Nginx requirements.

### Pros
- **Familiar Nginx Core**: Proven high-throughput reverse proxy performance.
- **Environment Variable Driven**: Configured via standard `VIRTUAL_HOST` environment variables.

### Cons & Trade-offs
- Requires two separate containers (`nginx-proxy` and `acme-companion`).
- Custom Nginx configurations or headers require mounting template files or volume overrides.

### Setup & Usage
```bash
# Start Nginx edge proxy
docker compose -f edge/docker-compose.nginx.yml up -d
```

### Application Container Environment (`apps/docker-compose.yml`)
```yaml
services:
  my-app:
    image: my-app:latest
    environment:
      - VIRTUAL_HOST=app.example.com
      - LETSENCRYPT_HOST=app.example.com
    networks:
      - oops_network
```

---

## Recommendation & Decision Guide

- **Choose Caddy (Default)** if you want the cleanest, zero-config setup with automatic SSL and minimal label clutter.
- **Choose Traefik** if you need a visual web dashboard, path-based routing, or advanced traffic management.
- **Choose Nginx Proxy** if your team mandates Nginx or already relies on `VIRTUAL_HOST` workflows.
