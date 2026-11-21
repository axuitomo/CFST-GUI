# CFST WebUI Bundle

This distribution includes `LICENSE`, `THIRD-PARTY-NOTICES.md`, `CA-CERTIFICATES-COPYRIGHT`, and `SOURCE.txt`.

## Docker Compose

1. Copy `.env.example` to `.env`.
2. Generate a token with `openssl rand -hex 24` and set `CFST_WEBUI_TOKEN` to it. There is no default token; compose refuses to start while it is empty.
3. Run `docker compose up -d --build`.
4. Open `http://localhost:34115` and enter the token.

Data is persisted in the `cfst-webui-data` Docker volume mounted at `/data`.
Application settings, scheduler rules, Cloudflare DNS push settings, GitHub export settings, exports, and backups are stored under `/data/data`.

Common commands:

```bash
docker compose up -d --build
docker compose ps
docker compose logs -f
docker compose restart
docker compose down
```

Host network mode is optional. It removes port publishing and lets the service listen directly on the host network:

```bash
docker compose -f docker-compose.yml -f docker-compose.host.yml up -d --build
```

Back up the named volume:

```bash
docker run --rm \
  -v cfst-webui-data:/data:ro \
  -v "$PWD:/backup" \
  busybox tar -czf /backup/cfst-webui-data.tar.gz -C /data .
```

Restore the named volume:

```bash
docker run --rm \
  -v cfst-webui-data:/data \
  -v "$PWD:/backup" \
  busybox sh -c 'cd /data && tar -xzf /backup/cfst-webui-data.tar.gz'
```

Run the published GHCR image instead of building locally:

```bash
TOKEN="$(openssl rand -hex 24)"; echo "CFST_WEBUI_TOKEN=$TOKEN"
docker run -d \
  --name cfst-webui \
  --restart unless-stopped \
  -p 34115:34115 \
  -e TZ=Asia/Shanghai \
  -e CFST_WEBUI_ADDR=0.0.0.0:34115 \
  -e CFST_WEBUI_TOKEN="$TOKEN" \
  -e CFST_GUI_PORTABLE_ROOT=/data \
  -e CFST_WEBUI_ALLOWED_ROOTS=/data \
  -v cfst-webui-data:/data \
  ghcr.io/axuitomo/cfst-gui:latest
```

## Local Linux

1. Run `./run-local.sh`.
2. Open `http://127.0.0.1:34115`.

`run-local.sh` keeps portable data under `./portable/data` by default and binds `127.0.0.1:34115`, where no token is needed. Override `CFST_WEBUI_ADDR` or `CFST_GUI_PORTABLE_ROOT` if you need a different bind address or storage path. Binding anything other than a loopback address also requires `CFST_WEBUI_TOKEN`, otherwise the service refuses to start.
