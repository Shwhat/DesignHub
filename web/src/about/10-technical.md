# Technical

## Architecture

| Part | Technology |
|---|---|
| Application server | Go, a single program serving the web interface and a JSON API |
| Web interface | Svelte and TypeScript, built with Vite and bundled into the server |
| Editors | Milkdown (Visual mode) and CodeMirror (Source, Split and Preview toggle modes) |
| Engineering repository | Git, through the go-git library. Markdown files with YAML front matter. |
| Database | PostgreSQL 17 |
| HTTPS | Caddy in front of the application, with a self-signed certificate until a domain is configured |
| Live updates | Server-Sent Events |
| Packaging | Docker Compose. Images for amd64 and arm64. |

## Updates

- A new version is installed by pulling the new image and restarting the containers.
- Database changes are applied automatically on startup.
- The version is shown at the top of this guide.

## Third-party components

| Component | Used for | License |
|---|---|---|
| go-git | Git repository | Apache 2.0 |
| pgx | PostgreSQL driver | MIT |
| Go crypto and term libraries | Password hashing, terminal input | BSD 3-Clause |
| ProtonMail go-crypto | Baseline signatures (OpenPGP) | BSD 3-Clause |
| yaml.v3 | Front matter | MIT and Apache 2.0 |
| Svelte | Web interface | MIT |
| Milkdown | Visual editor | MIT |
| CodeMirror | Source editor | MIT |
| unified, remark, rehype | Markdown rendering and sanitizing | MIT |
| SortableJS | Drag and drop | MIT |
| PostgreSQL | Database | PostgreSQL License |
| Caddy | HTTPS proxy | Apache 2.0 |
| Alpine Linux | Container base | Various open source licenses |
