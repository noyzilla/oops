# Edge Stack (`compose.edge.yaml`) — Reverse Proxy & Auto-SSL Guide

The Edge stack (`compose.edge.yaml`) terminates public HTTP/HTTPS traffic, handles automatic SSL certificates (Let's Encrypt / ZeroSSL / Local Root CA), and routes incoming requests to internal container workloads over `oops_edge`.

This guide compares the 3 reverse proxy options you can run in `compose.edge.yaml`:

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
# Start default Caddy edge proxy & oops daemon (via x-oops-stack in compose.yaml)
oops up

# Or target the edge stack directly
oops up /edge
```

### Application Container Labels (`compose.apps.yaml`)
```yaml
services:
  my-app:
    image: my-app:latest
    labels:
      # Caddy Auto-SSL Reverse Proxy
      caddy: "app.web.oops"
      caddy.reverse_proxy: "{{upstreams 80}}"
    networks:
      - oops_edge
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
# Define traefik service in compose.edge.yaml (or compose.edge.local.yaml) and add to x-oops-stack:
oops up /edge
```

### Application Container Labels (`compose.apps.yaml`)
```yaml
services:
  my-app:
    image: my-app:latest
    labels:
      traefik.enable: "true"
      traefik.http.routers.myapp.rule: "Host(`app.web.oops`)"
      traefik.http.routers.myapp.entrypoints: "websecure"
      traefik.http.routers.myapp.tls.certresolver: "letsencrypt"
    networks:
      - oops_edge
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
# Define nginx-proxy and acme-companion in compose.edge.yaml and add to x-oops-stack:
oops up /edge
```

### Application Container Environment (`compose.apps.yaml`)
```yaml
services:
  my-app:
    image: my-app:latest
    environment:
      - VIRTUAL_HOST=app.web.oops
      - LETSENCRYPT_HOST=app.web.oops
    networks:
      - oops_edge
```

---

## Local TLS Root CA Trust (`oops box cert`)

For local wildcard domains (`https://*.web.oops`), run:
```bash
oops box cert
```
- Installs the local Caddy Root CA certificate into the host OS trust store (macOS System Keychain or Linux `ca-certificates`).
- **Chrome / Safari / Edge / curl**: Works immediately without warnings.
- **Firefox / Firefox Developer Edition**: Firefox uses an isolated certificate store. To trust the OS Keychain:
  1. Open `about:config`
  2. Set `security.enterprise_roots.enabled` to `true`

---

## Recommendation & Decision Guide

- **Choose Caddy (Default)** if you want the cleanest, zero-config setup with automatic SSL and minimal label clutter.
- **Choose Traefik** if you need a visual web dashboard, path-based routing, or advanced traffic management.
- **Choose Nginx Proxy** if your team mandates Nginx or already relies on `VIRTUAL_HOST` workflows.
