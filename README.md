# DesignHub

Self-hosted engineering documentation, work tracking and Bill of Design in one connected application.

- **Pages:** documentation in spaces, with a page tree, four editing modes (Visual, Split, Source, Preview toggle), comments, history and controlled pages.
- **Work:** tasks, issues, nonconformances, actions and epics, with boards, lists and My Work.
- **Bill of Design:** parts, assemblies, requirements, procedures and documents with numeric revisions, multi-level BOMs, where-used, traceability, validation and change orders with electronic signatures and signed baselines.

DesignHub runs entirely on the local network. Nothing is loaded from the internet at runtime.

## Documentation

| Document | Purpose |
|---|---|
| [docs/SPEC.md](docs/SPEC.md) | Product specification and release plan |
| [docs/DATA-MODEL.md](docs/DATA-MODEL.md) | Identification, links, revisions, storage and field lists |
| [docs/SCREENS.md](docs/SCREENS.md) | Version 1 screen layouts |
| [docs/FRONTEND.md](docs/FRONTEND.md) | Frontend framework and editor selection |
| [docs/TEST-PLAN.md](docs/TEST-PLAN.md) | Version 1 test plan and seed data |
| [docs/HANDOFF.md](docs/HANDOFF.md) | Project rules, current state and build notes |

## Server requirements

| | Minimum | Recommended |
|---|---|---|
| CPU | 2 cores | 4 cores |
| Memory | 4 GB | 8 GB |
| Disk | 32 GB | 64 GB or more, depending on attachments |
| Software | Linux with Docker Engine and the Docker Compose plugin | |
| Network | Fixed IP address on the local network | |

Images are published for amd64 and arm64.

## Install

1. On the Docker host, create a folder and save [docker-compose.yml](docker-compose.yml) in it.
2. Start DesignHub:

   ```
   docker compose up -d
   ```

   The first start creates the database password, a self-signed HTTPS certificate and the baseline signing key, creates the database and applies its schema. No manual database steps are needed.

3. Create the first Super Admin. The command asks for the user name and password in the terminal; the password is never stored in a file.

   ```
   docker compose exec -it designhub designhub setup-admin
   ```

4. Open `https://<server address>` in a browser and sign in. The browser shows a certificate warning once per device until a domain with a trusted certificate is set up.
5. In Admin, System, enter the addresses DesignHub is reached by (for example the server's IP address) under Allowed addresses. Requests for any other host name are then refused.

The same `setup-admin` command resets the password of an existing Super Admin.

## Update

```
docker compose pull && docker compose up -d
```

Database changes are applied automatically on startup. Each release is also tagged with its version number, for example `ghcr.io/shwhat/designhub:1.0.0`.

## Test data

A test installation can be loaded with the seed data set in [docs/TEST-PLAN.md](docs/TEST-PLAN.md) Appendix A. Do not load it on a production installation.

```
docker compose exec -it designhub designhub seed
```

## Backup and restore

Backups of the database, the engineering repository and attachments are written to the `backups` volume every night at 02:00 and the last 14 are kept. The schedule is changed in Admin, System. A backup can also be started there, or from the command line:

```
docker compose exec designhub designhub backup
docker compose exec designhub designhub backups
```

To restore a backup:

1. Stop the application, leaving the database running:

   ```
   docker compose stop designhub proxy
   ```

2. Run the restore with the backup name from `designhub backups`:

   ```
   docker compose run --rm -it designhub restore designhub-YYYYMMDD-HHMMSS
   ```

   The current engineering repository is kept beside the restored one as `repo.before-restore-<time>`.

3. Start everything again:

   ```
   docker compose up -d
   ```

   The search index is rebuilt automatically on startup.

Proxmox virtual machine backups are recommended as a second layer.

## HTTPS with a domain

When DesignHub has a domain name, edit the `caddyfile` section of `docker-compose.yml`: replace `:443` with the domain name and delete the `tls` line. The proxy then obtains and renews a trusted certificate itself. Add the domain to Allowed addresses in Admin, System.

## Settings

Environment variables of the `designhub` service:

| Variable | Default | Meaning |
|---|---|---|
| `DESIGNHUB_SECURE_COOKIE` | `auto` | `auto` marks the session cookie secure when the request arrives over HTTPS; `always` or `never` force it |
| `DESIGNHUB_DB_HOST` | `postgres` | Database host |
| `DESIGNHUB_DB_PORT` | `5432` | Database port |
| `DESIGNHUB_DB_NAME` | `designhub` | Database name |
| `DESIGNHUB_DB_USER` | `designhub` | Database user |
| `DESIGNHUB_DB_PASSWORD` | From `/secrets/db_password` | Database password |
| `DESIGNHUB_DATABASE_URL` | | Full PostgreSQL connection URL; replaces the separate database settings |
| `DESIGNHUB_ADDR` | `:8080` | Address and port inside the container |
| `DESIGNHUB_REPO` | `/data/repo` | Engineering git repository |
| `DESIGNHUB_FILES` | `/data/files` | Attachment file store |
| `DESIGNHUB_BACKUPS` | `/backups` | Backup output |
| `DESIGNHUB_SECRETS` | `/secrets` | Generated secrets |
| `TZ` | `America/Los_Angeles` | Time zone for scheduled backups |

## Data storage

| Volume | Contents |
|---|---|
| `repo` | Engineering git repository (bare) and attachment file store |
| `pgdata` | PostgreSQL data |
| `backups` | Backups |
| `secrets` | Database password, TLS certificate and baseline signing key |

The engineering repository is a standard git repository. Items and pages are markdown files with YAML frontmatter, readable without DesignHub. Each change order works on its own branch, and each release creates a signed `baseline/CO-NNNN` tag.

## Commands

```
designhub serve          Run the server (default)
designhub init-secrets   Create the database password, TLS certificate and signing key if missing
designhub setup-admin    Create a Super Admin account, or reset a Super Admin password
designhub seed           Load the test data set (test installations only)
designhub backup         Write a backup now
designhub restore NAME   Restore a backup (stop the designhub service first)
designhub backups        List backups
designhub healthcheck    Exit 0 when the server answers
designhub version        Print the version
```

## Build from source

Build the image from a clone of this repository:

```
docker build -t ghcr.io/shwhat/designhub:latest .
```

Or build the parts separately. Go 1.24 and Node.js 22 are needed at build time only; Go dependencies are vendored.

```
cd web && npm ci && npm run build && cd ..
CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags "-s -w" -o designhub ./cmd/designhub
```

Run the tests against a PostgreSQL test database:

```
cd web && npm run check && cd ..
DESIGNHUB_TEST_DATABASE_URL=postgres://user:password@localhost:5432/designhub_test?sslmode=disable go test -mod=vendor ./...
```

GitHub Actions runs these tests on every push and pull request, and publishes the image to `ghcr.io/shwhat/designhub` on every push to `main`.

## Source layout

| Path | Purpose |
|---|---|
| `cmd/designhub/` | Command-line entry point: server, setup, seed, backup and restore |
| `internal/app/` | HTTP API, sessions, permissions, items, pages, change orders, work, search, notifications, backups and seed data |
| `internal/model/` | Item format, identifiers, Bill of Design rollup and validation |
| `internal/repo/` | Engineering git repository: commits, change order branches, merges and signed tags |
| `internal/db/` | PostgreSQL connection and migrations |
| `internal/auth/` | Argon2id password hashing and tokens |
| `internal/config/` | Settings from environment variables |
| `internal/secrets/` | Generated secrets |
| `web/` | Svelte frontend, built into `web/dist` and embedded in the server binary |
| `vendor/` | Vendored Go dependencies |
| `Dockerfile` | Multi-architecture image |
| `docker-compose.yml` | Deployment: application, PostgreSQL and HTTPS proxy |
