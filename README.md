# Oops

![Oops Logo](logo.jpg)
> *Because "Oops, sorry!" is the most common phrase when DevOps breaks production.*

Oops is a lightweight service written in Go (Golang) designed to handle webhooks from CI/CD systems (e.g., GitHub Actions, GitLab CI). It automates the process of pulling the latest Docker images, performing graceful shutdowns, recreating containers, and pruning old images, all based on Docker Labels defined on the target containers.

## Features

- **Zero-Downtime Architecture (Image Mode)**: Pulls the latest image and orchestrates container recreation with exact volume and network preservation.
- **Source-Based Deployment (Git Mode)**: Supports lightweight environments by spawning a temporary container to `git pull` the latest source code directly into your mounted volumes, then restarts the target container.
- **Regex Container Targeting**: Update specific containers dynamically using Regular Expressions via the webhook payload (e.g., target `webapp_.*`).
- **Per-Project Security**: Assign specific Secrets to individual containers via labels for isolated, project-level security. Each container strictly validates its own webhook token.
- **Graceful Shutdown (Stop Command)**: Execute custom commands (e.g., `sleep`, `php artisan horizon:terminate`) inside the container before stopping it, preventing interrupted background jobs.
- **Auto Cleanup**: Automatically removes dangling images after a successful Image deployment to save disk space.

---

## Installation & Setup

You can install the Oops using the official Image or by building it from source.

### Method 1: Using an Image (Recommended)

1. Create a `docker-compose.yml` for the Oops on your server:
   ```yaml
   services:
     oops:
       image: ghcr.io/noyzilla/oops:latest
       container_name: oops
       restart: unless-stopped
       volumes:
         # Must mount docker.sock to allow the oops to manage other containers
         - /var/run/docker.sock:/var/run/docker.sock
       ports:
         - "8080:80"
   ```
2. Start the service:
   ```bash
   docker compose up -d
   ```

### Method 2: Building from Source

1. Clone this repository:
   ```bash
   git clone https://github.com/noyzilla/oops.git
   cd oops
   ```
2. Start the service and build the image locally:
   ```bash
   docker compose up -d --build
   ```

> The Oops will listen for webhooks on port `8080`.

---

## Target Container Configuration

For security reasons, the Oops relies exclusively on **Docker Labels** instead of Environment Variables. This prevents sensitive infrastructure secrets from being exposed inside the container's application space (preventing leaks in case of a Remote Code Execution vulnerability in the app).

To allow the Oops to manage a specific container, the following labels must be added to the target project's `docker-compose.yml`:

| Label | Required | Description |
| --- | --- | --- |
| `oops.enable` | Yes | Must be set to `true` to authorize the Oops to manage this container. |
| `oops.secret` | Yes | Project-specific secret. The webhook token must match this secret exactly for the update to proceed. |
| `oops.stop.cmd` | No | **(Optional)** Command to execute inside the container **before** it is stopped (useful for graceful shutdowns). |
| `oops.stop.timeout` | No | **(Optional)** Maximum time (in seconds) to wait for the stop command to complete. Default is `60` seconds. |
| `oops.git.dir` | **Yes (Git Mode)** | The absolute path inside the container where the source code is mounted (e.g. `/app`). |
| `oops.tool.image` | No | **(Optional)** Image to run as a tool container after git pull. |
| `oops.tool.cmd` | No | **(Optional)** Command to run inside the tool container. |

### Example Target `docker-compose.yml`:
```yaml
services:
  app_image_node:
    image: myrepo/myimage:latest
    container_name: web_image_node
    labels:
      - "oops.enable=true"
      - "oops.secret=${MY_APP_SECRET}" # Strongly recommended to use .env file for secrets
      - "oops.stop.cmd=sleep 5 && echo 'Finishing background jobs...'"
      - "oops.stop.timeout=30"

  app_git_php:
    image: dunglas/frankenphp:latest-alpine
    container_name: web_git_php
    volumes:
      - ./sites/web_git_php/app:/app/public
    labels:
      - "oops.enable=true"
      - "oops.secret=my-secret"
      - "oops.git.dir=/app/public"
      - "oops.tool.image=composer:latest"
      - "oops.tool.cmd=composer install --no-dev"

  app_git_bun:
    image: oven/bun:alpine
    container_name: web_git_bun
    command: ["bun", "run", "index.ts"]
    volumes:
      - ./sites/web_git_bun/app:/app
    labels:
      - "oops.enable=true"
      - "oops.secret=my-bun-secret"
      - "oops.git.dir=/app"
      - "oops.tool.image=oven/bun:alpine"
      - "oops.tool.cmd=bun install"
```

---

## Triggering Deployments (Webhook)

The service listens for HTTP `POST` requests at the `/update` endpoint. You must provide the authentication token in the headers and the target configuration in a JSON payload.

### 1. Header (Authentication)
The `Authorization` header is required.
```http
Authorization: Bearer <SECRET_TOKEN>
```

### 2. JSON Payload

You can trigger different deployment modes by specifying the `action` field (`image` or `git`).

#### A. Image Deployment Mode (Default)
The `image` field is **mandatory** for this mode.

- **Update all containers using a specific image**
  ```json
  {
    "action": "image",
    "image": "myrepo/myimage:latest"
  }
  ```

- **Update multiple containers matching a Regex**
  ```json
  {
    "action": "image",
    "image": "myrepo/myimage:latest",
    "container": "^web_image_.*"
  }
  ```

#### B. Git Pull Deployment Mode
Uses a temporary container to execute `git fetch` and `git checkout` to a specific tag in your mounted volume, then restarts the target container.

> [!WARNING]
> **Data Loss Prevention:** The oops uses `git checkout -f` to enforce the tag state. Any modifications to tracked files inside the `git_dir` **will be overwritten**. For production, always mount persistent data (like user uploads or SQLite databases) as a separate Docker Volume.

- **Deploy a specific Git Tag (Required)**
  ```json
  {
    "action": "git",
    "tag": "v1.2.3",
    "container": "^web_git_bun$"
  }
  ```

### Example cURL Request:
```bash
curl -X POST \
  -H "Authorization: Bearer super-secret-key-for-this-app" \
  -H "Content-Type: application/json" \
  -d '{"action": "image", "image": "myrepo/myimage:latest", "container": "^web_image_.*"}' \
  http://<SERVER_IP>:8080/update
```

---

## Supported Payload Fields

- `action`: (Required) Either `"image"` or `"git"`. Defaults to `"image"`.
- `image`: (Required for `image` action) The full image name and tag to deploy.
- `tag`: (Required for `git` action) The specific Git tag to checkout (e.g., `v1.2.3`).
- `container`: (Optional) Regular expression matching the container names to update. If omitted, it will attempt to update ALL authorized containers matching the given `image`.
