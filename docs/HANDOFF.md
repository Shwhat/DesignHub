# DesignHub Handoff

Date: 2026-10-30 (updated 21:50)
Purpose: everything needed to continue DesignHub in a new conversation. This document replaces all earlier handoff documents.

## Start here

Paste this into the new chat:

> I'm continuing work on DesignHub. Read docs/HANDOFF.md in GitHub repo Shwhat/DesignHub, branch main, then docs/SPEC.md, docs/DATA-MODEL.md, docs/SCREENS.md, docs/FRONTEND.md and docs/TEST-PLAN.md. Version 1 is merged and installed on my VM, and testing has started. The next step is the bug list in the handoff. Start by asking me about any bug list item that is incomplete.

## Documents

| Document | Contents | State |
|---|---|---|
| `docs/SPEC.md` | Product specification, decisions, v1/v2/v3 release plan | v0.2, complete |
| `docs/DATA-MODEL.md` | Identification, links, revisions, storage, field lists | v0.1, complete |
| `docs/SCREENS.md` | General layout and the 15 v1 screens | v0.1, complete |
| `docs/TEST-PLAN.md` | v1 test plan: checklist, phone and tablet checks, scenarios, seed data | v0.1, complete |
| `docs/FRONTEND.md` | Frontend framework and editor selection | v0.1, complete |
| `docs/HANDOFF.md` | This document: rules, state, next steps, technical notes | Current |

All documents and the v1 code are on `main` (PR #2, merged 2026-10-30).

## Project rules

- Do not build, compile or package the app until Devon says so. Writing code and docs is allowed when asked; building is not.
- Never use em dashes in any output, including code comments, UI text and docs.
- The app shows only DesignHub branding. Only exception: the Google sign-in button.
- Do not mention or include EPLAN.
- No Python anywhere in the product.
- Answer questions simply, without extra unrequested actions.
- Deliverable documents contain no tips or meta-commentary.
- When reviewing a series of items, show progress as "N of M".
- Work on branches. Devon reviews and merges into `main`.

## Order of work

| Step | State |
|---|---|
| 1. Specification | Complete |
| 2. Data model | Complete |
| 3. Screen layouts | Complete |
| 4. v1 test plan: what Devon will test and what "working" means | Complete |
| 5. Proxmox host preparation | Not needed. DesignHub installs on an existing Docker host like OpenSchem. |
| 6. Build v1 on branch `v1-build`, one pull request at the end | Complete (PR #2) |
| 7. Devon merges PR #2 and installs on the VM | Complete |
| 8. Devon tests with TEST-PLAN.md | In progress |
| 9. Fix the bug list, one branch per fix or group of fixes | Next |

## Key decisions (detail in SPEC.md, DATA-MODEL.md and SCREENS.md)

**Platform**
- Server version only: Docker Compose on a Proxmox VM, local network first, domain later.
- Go backend, bundled frontend, no runtime dependence on the internet.
- Engineering data and pages in git as markdown. Work items, change order records, users, comments, notifications, lists and audit log in PostgreSQL. Attachments in a file store referenced from git. InfluxDB in v3.

**Identification**
- Production parts and assemblies: 6-digit base plus 3-digit category suffix (100042-001). Suffixes: -000 layout, -0XX and -1XX piece part, -2XX assembly, -3XX harness, -4XX PCB, -5XX PCBA, -6XX internal electronic component, -9XX interface or make-on-assembly.
- The next number is offered automatically. The user may change base and suffix if the full number is unused. The counter skips any base already used in any form.
- Reference parts: 7-digit (1000042), convertible to production.
- COTS parts: manufacturer code, period, order number (PXC.3031212). Manufacturer list in v1.
- Everything else: PREFIX-NUMBER (REQ, PRC, DOC, PG, CO, SP, TSK, ISS, NCR, ACT, EPC).
- Never-released drafts can be deleted; numbers are never reused.

**Revisions and control**
- Numeric revisions, first release 01. No revision before first release.
- Drafts change freely. Release and every later revision go through a change order.
- The owner starts a new revision when ready; the change order releases it.
- Engineering statuses: Draft, In Review, Released, Superseded, Obsolete.

**Organization and access**
- Engineering items and change orders live in a company-wide library with a home space. Pages and work items belong to one space.
- Each user has one library role and one role per space.
- Any record can be locked to an access list; others see nothing.

**Screens**
- Top bar (search, notifications, profile menu) and left sidebar (Home, My Work, Library, Search, space switcher, page tree, Work entries).
- Responsive for desktop, tablet and phone. Light and dark themes by user setting. Comfortable density.
- Records share one layout: header with Actions menu, tabs, comment side panel, edit in place with autosaved drafts.
- Sign-in: "Keep me signed in" lasts 1 day.

## Test plan decisions (detail in TEST-PLAN.md)

| Question | Decision |
|---|---|
| Tester | Devon only |
| Environment | The Proxmox VM, which already has Docker installed |
| Format | Both: a checklist per screen and feature, and end-to-end scenarios |
| Test data | Both: seed data set for the checklist, realistic factory build-out sample for the scenarios |
| Pass criteria | Must checks must pass. Failed Should checks are logged as GitHub issues with label `v1-test`. |
| Phone and tablet | Key screens only |

## Open items

| Item | Owner |
|---|---|
| Meaning of "major update" for the Home layout reset (SCREENS.md section 17) | To be defined |
| Domain name and HTTPS certificate (needed for Google sign-in) | Devon |
| Mail server for email notifications (v2) | Devon |
| Real items for the realistic sample (TEST-PLAN.md Appendix B) | Devon |
| Bug 4 in the bug list is incomplete ("where") | Devon |

## Bug list

Reported by Devon on 2026-10-30 after installing v1 on the VM. Findings are from reproducing on a local server with the same build.

| # | Report | Finding | State |
|---|---|---|---|
| 1 | Pages do not show up in the tree when created. A much more robust pages setup is needed. | Not reproduced. Creating a page with the sidebar "+" button (top level) and with "New child page" adds it to the tree immediately, on a fresh install and on the seed data; the server returns it in `GET /api/spaces/{id}/tree`. The sidebar "+" and "..." menu are the only ways to create a page. Ask Devon how the pages were created (screen, button, space) and whether they appear after a browser reload. The "more robust pages setup" request needs scoping with Devon: for example a New page button on the space home and page view, a page picker for the parent, templates, a full page list per space. | Open |
| 2 | Parts do not show under Library. The item count goes up. | Reproduced: the table shows "10 of 10" with no columns or rows. Cause: `GET /api/prefs/{key}` returns `{"ok":true}` when the user has no saved preference (`apiGetPref` in `internal/app/users.go` calls `ok(w, nil)`), so `web/src/pages/Library.svelte` uses that object as its column list and draws no columns. Affects every user who has not saved Library columns, on every Library tab. Fix: return JSON `null` when no preference exists, and check `Array.isArray` in Library before using a saved column list. Check other `/api/prefs` readers for the same assumption. | Open, cause found |
| 3 | The whole app looks very basic. | Design pass needed across all screens: typography, spacing, color, icons (currently text glyphs such as ▶ and ⋯), headers, tables, cards and empty states. Agree a visual direction with Devon first, using the frontend-design guidance. | Open |
| 4 | "where" (message cut off) | Incomplete. Ask Devon for the rest. | Open, needs detail |

### Other known issues

| Issue | Where |
|---|---|
| The four editing modes, image paste and "/" block insert have not been driven in a browser; only page display is verified | `web/src/components/Editor.svelte` |
| Search icon in the phone top bar is very small | `web/src/components/TopBar.svelte` |
| Spacing around the baseline link in the released change order banner | `web/src/pages/ChangeOrderView.svelte` |
| 22 svelte-check warnings (state captured at first value); harmless today because pages remount on route change | `web/src` |
| `web/dist/.gitkeep` is deleted by every frontend build; restore it before committing | `web/vite.config.ts` |

## Build decisions (2026-10-30)

| Item | Decision |
|---|---|
| Base | Build from the specs on branch `v1-build`, created from `docs/spec-v1`. PR #1 is not waited on. |
| Git | Full git library: go-git v5 |
| Two-factor | Not in v1. Provided by Google sign-in in v2. |
| Inactivity sign-out | 30 minutes |
| Rejected or cancelled change order | Affected items keep their status. Admin setting can return them to Draft. |
| Approvals | Every assigned approver must approve |
| Install | Like OpenSchem: images on `ghcr.io/shwhat/designhub` published by GitHub Actions, `docker compose up -d` from the published compose file |
| First Super Admin | Setup command on the host, password typed in the terminal |
| HTTPS | Self-signed certificate by default, changeable after install |
| Repository | Public |
| Frontend | Svelte 5, Vite, Milkdown and CodeMirror 6 (FRONTEND.md) |

## Architecture

| Part | Choice |
|---|---|
| Backend | Go 1.24, single binary, JSON API, frontend embedded in the binary |
| Frontend | Svelte 5 and TypeScript built with Vite; Milkdown (Visual) and CodeMirror 6 (Source, Split, Preview toggle). Bundled assets only. |
| Engineering data | Markdown files with YAML frontmatter in a bare git repository (go-git v5) on the `repo` volume. Change orders on `co/CO-NNNN` branches; releases merge to `main` and create signed `baseline/CO-NNNN` tags. |
| App data | PostgreSQL 17 (pgx v5) |
| Time-series | InfluxDB in v3 of the release plan |
| Packaging | Docker Compose: `init` (secrets), `designhub`, `postgres`, `proxy` (Caddy, self-signed HTTPS) |
| Images | `ghcr.io/shwhat/designhub`, amd64 and arm64, published by GitHub Actions on every push to `main` |
| Sign-in | Local accounts with Argon2id in v1; Google sign-in with two-factor in v2 once a domain with HTTPS exists |

## Current code on `main`

All nine build parts are implemented: server foundation, sign-in and admin, spaces and pages, engineering items, Bill of Design, change orders, work and NCRs, search and notifications, backup and restore.

| Path | Contents |
|---|---|
| `cmd/designhub/` | Commands: `serve`, `init-secrets`, `setup-admin`, `seed`, `backup`, `restore`, `backups`, `healthcheck`, `version` |
| `internal/app/` | HTTP API and business rules: sessions, CSRF, permissions, record locking, items and numbering, BOM, pages, change orders and signatures, work items and NCR workflow, notifications, live updates (Server-Sent Events), search, audit log, backups, scheduler, seed data |
| `internal/model/` | Item format, identifiers, BOM rollup, validation |
| `internal/repo/` | Git repository: commits, branches, merges, signed tags, history |
| `internal/db/` | Connection and migrations (`migrations/0001_schema.sql`, `0002_initial_data.sql`) |
| `internal/auth/`, `internal/config/`, `internal/secrets/` | Password hashing, environment settings, generated secrets |
| `web/` | Svelte frontend: `src/pages/` one file per screen, `src/components/` shared parts, `src/lib/` API client, router and state |
| `Dockerfile`, `docker-compose.yml`, `.github/workflows/docker.yml` | Image, deployment and CI |

### Verification done in the build session

- `go vet` clean; Go tests pass against PostgreSQL (`internal/app`, `internal/model`, `internal/repo`).
- Frontend type-check has no errors; production build succeeds.
- Seed data loaded on a local server matches TEST-PLAN.md Appendix A: next production number 100008-001, flat total of 100001-001 in 100004-201 is 10, traceability coverage, My Work order, mention notification.
- Access checks: locked DOC-0002 and SP-0002 pages return not found for `editor2`; locked BOM line hidden; Viewer edits refused; missing CSRF token refused; foreign Host refused (421) once Allowed addresses is set.
- CO-0002 approved with password signature and released: 100002-001 rev 02 Released, rev 01 Superseded, signed baseline tag, `git fsck --strict` passes.
- Backup, then restore of database, repository and attachments.
- Key screens load in headless Chromium on desktop and phone width with no browser errors.

### Install state

- PR #2 merged into `main` on 2026-10-30; GitHub Actions published `ghcr.io/shwhat/designhub:latest`.
- Devon installed it on the VM with Docker. The Super Admin is created with `docker compose exec -it designhub designhub setup-admin`; the install does not prompt for an account.

### Next steps

1. Get the missing detail for bug 1 and bug 4 from Devon, and agree the scope of "more robust pages" and the visual direction for bug 3.
2. Fix bug 2 first; it blocks the Library checks in TEST-PLAN.md.
3. Fix bugs on branches from `main`, open a pull request for Devon to merge, then Devon pulls the new image (`docker compose pull && docker compose up -d`).
4. Devon continues TEST-PLAN.md and adds failures to the bug list or as GitHub issues with label `v1-test`.

## Build environment notes

These apply to Claude Code cloud sessions:

- Go 1.24, Node.js 22 and PostgreSQL 16 are available. Start PostgreSQL with `pg_ctlcluster 16 main start` for the Go tests (`DESIGNHUB_TEST_DATABASE_URL`).
- proxy.golang.org is blocked. All Go dependencies, including golang.org/x modules, are vendored; build with `-mod=vendor`.
- The npm registry is reachable. Docker is not available in the session, so images are built by GitHub Actions.
- Playwright browser downloads are blocked; the `@sparticuz/chromium` npm package provides a headless Chromium for screen checks.
- `setup-admin`, `seed` and `restore` require an interactive terminal.

```
cd web && npm ci && npm run build && cd ..
CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags "-s -w" -o designhub ./cmd/designhub
```
