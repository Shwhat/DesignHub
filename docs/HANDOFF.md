# DesignHub Handoff

Date: 2026-10-31 (updated 11:30)
Purpose: everything needed to continue DesignHub in a new conversation. This document replaces all earlier handoff documents.

## Start here

Paste this into the new chat:

> I'm continuing work on DesignHub. Read docs/HANDOFF.md in GitHub repo Shwhat/DesignHub, branch main, then docs/SPEC.md, docs/DATA-MODEL.md, docs/SCREENS.md, docs/FRONTEND.md and docs/TEST-PLAN.md. Version 1 is installed on my VM and testing is in progress. PR #4 (bug 2 and four smaller fixes) is merged. The next step is the open bug list in the handoff. Start by asking me the open questions listed there.

## Documents

| Document | Contents | State |
|---|---|---|
| `docs/SPEC.md` | Product specification, decisions, v1/v2/v3 release plan | v0.3, complete |
| `docs/DATA-MODEL.md` | Identification, links, revisions, storage, field lists | v0.2, complete |
| `docs/SCREENS.md` | General layout and the 15 v1 screens | v0.2, complete |
| `docs/TEST-PLAN.md` | v1 test plan: checklist, phone and tablet checks, scenarios, seed data | v0.2, complete |
| `docs/FRONTEND.md` | Frontend framework and editor selection | v0.1, complete |
| `docs/HANDOFF.md` | This document: rules, state, bug list, next steps, technical notes | Current |

All documents and code are on `main`.

## Project rules

- Do not build, compile or package the app until Devon says so. Writing code and docs is allowed when asked; building is not.
- Exception approved 2026-10-31: Go tests (`go vet`, `go test`) and the frontend type-check (`svelte-check`) may be run to verify fixes. A frontend production build (`npm run build`) and the Go binary build still need Devon's approval.
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
| 9. Fix the bug list, one branch per fix or group of fixes | In progress. PR #4 merged. Bugs 1, 3 and 4 wait on Devon. |

## Pull request history

| PR | Title | Merged |
|---|---|---|
| #1 | DesignHub specification v0.1 | 2026-10-29 |
| #2 | DesignHub version 1 | 2026-10-30 |
| #3 | Update handoff with install state and bug list | 2026-10-31 |
| #4 | Fix Library columns, phone search icon, banner spacing and svelte-check warnings | 2026-10-31 |

The GitHub Actions "Docker image" workflow succeeded for the PR #4 merge commit (`62238d1`), so `ghcr.io/shwhat/designhub:latest` contains the fixes.

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

No `v1-test` issues have been logged yet.

## Open items

| Item | Owner |
|---|---|
| Meaning of "major update" for the Home layout reset (SCREENS.md section 17) | To be defined |
| Domain name and HTTPS certificate (needed for Google sign-in) | Devon |
| Mail server for email notifications (v2) | Devon |
| Real items for the realistic sample (TEST-PLAN.md Appendix B) | Devon |
| Answers to the open bug questions below | Devon |

## Bug list

Reported by Devon on 2026-10-30 after installing v1 on the VM. Findings are from reproducing on a local server with the same build.

| # | Report | Finding | State |
|---|---|---|---|
| 1 | Pages do not show up in the tree when created. A much more robust pages setup is needed. | Not reproduced. Creating a page with the sidebar "+" button (top level) and with "New child page" adds it to the tree immediately, on a fresh install and on the seed data; the server returns it in `GET /api/spaces/{id}/tree`. The sidebar "+" and "..." menu are the only ways to create a page. | Open, waiting on Devon (questions below) |
| 2 | Parts do not show under Library. The item count goes up. | Cause: `GET /api/prefs/{key}` returned `{"ok":true}` when the user had no saved preference, so Library used that object as its column list and drew no columns. The work List view had the same fault. | Fixed in PR #4. Devon to retest. |
| 3 | The whole app looks very basic. | Design pass needed across all screens: typography, spacing, color, icons (text glyphs such as ▶ and ⋯ are still used in many places), headers, tables, cards and empty states. FRONTEND.md section 7 already names Lucide icons and the Inter font as the intended bundled assets. | Open, waiting on Devon to agree a visual direction |
| 4 | "where" (message cut off) | Incomplete | Open, waiting on Devon |

### Open questions for Devon

These were asked on 2026-10-31 and not yet answered.

| Bug | Question |
|---|---|
| 1 | How was the page created: the sidebar "+", the "..." menu "New child page", or somewhere else? |
| 1 | Which space was it in? |
| 1 | Did the page appear after a browser reload? |
| 1 | What does "a much more robust pages setup" mean? Candidates: a New page button on the space home and page view, a parent page picker, templates, a full page list per space. |
| 3 | Which visual direction? Agree this before any redesign work. |
| 4 | What was the rest of the message after "where"? |

### PR #4 detail

| Item | Change | Files |
|---|---|---|
| Bug 2 | `apiGetPref` answers JSON `null` when no row exists. Library and WorkList use a saved column list only if it is a non-empty array. BOMView uses a saved collapse state only if it is an object. New test `TestPrefsUnsetIsNull` fails on the old code and passes on the fix. | `internal/app/users.go`, `internal/app/app_test.go`, `web/src/pages/Library.svelte`, `web/src/pages/WorkList.svelte`, `web/src/components/BOMView.svelte` |
| Phone search icon | Text glyph replaced with a 20 px SVG magnifier matching the bell icon | `web/src/components/TopBar.svelte` |
| Banner spacing | `.banner` changed from flex to normal text flow, so links and punctuation no longer get gaps. Fixes the released change order banner and the same gaps in item and page banners. Buttons inside banners get a small left margin. | `web/src/app.css` |
| svelte-check warnings | All 22 cleared. One-time prop reads wrapped in `untrack`; behavior is unchanged because `App.svelte` remounts the page in `{#key route.path}`. Files `input` declared as state. `tabindex="-1"` on the menu. | 17 files under `web/src` |
| `.gitkeep` | A Vite plugin writes `web/dist/.gitkeep` after every build | `web/vite.config.ts` |

Not yet verified: the `.gitkeep` plugin, because no frontend build has been run since. Check that `web/dist/.gitkeep` exists after the next approved build.

### Retest after pulling the PR #4 image

On the VM: `docker compose pull && docker compose up -d`, then:

| Check | What to confirm |
|---|---|
| ITM-01 | Library tabs and part columns show, with rows |
| ITM-02 | Superseded and Obsolete toggle |
| ITM-03 | Column choice kept after sign out and in |
| ITM-04 | Saved filter kept |
| WRK-06 | Work List shows columns; filter and column choice kept |
| ASM-13 | BOM tree expansion remembered |
| CO-12, CO-13 | Released change order banner and superseded revision banner read normally |
| RSP-01 to RSP-11 | Phone top bar search icon size |

### Other known issues

| Issue | Where |
|---|---|
| The four editing modes, image paste and "/" block insert have not been driven in a browser; only page display is verified | `web/src/components/Editor.svelte` |
| `.gitkeep` plugin not yet exercised by a build | `web/vite.config.ts` |

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
| `internal/app/` | HTTP API and business rules: sessions, CSRF, permissions, record locking, items and numbering, BOM, pages, change orders and signatures, work items and NCR workflow, notifications, live updates (Server-Sent Events), search, audit log, backups, scheduler, seed data, user preferences (`users.go`) |
| `internal/model/` | Item format, identifiers, BOM rollup, validation |
| `internal/repo/` | Git repository: commits, branches, merges, signed tags, history |
| `internal/db/` | Connection and migrations (`migrations/0001_schema.sql`, `0002_initial_data.sql`) |
| `internal/auth/`, `internal/config/`, `internal/secrets/` | Password hashing, environment settings, generated secrets |
| `web/` | Svelte frontend: `src/App.svelte` routes and remounts each page per path, `src/pages/` one file per screen, `src/components/` shared parts, `src/lib/` API client, router and state, `src/app.css` shared styles |
| `Dockerfile`, `docker-compose.yml`, `.github/workflows/docker.yml` | Image, deployment and CI |

### Frontend notes

- User preferences are stored per user and key with `GET` and `PUT /api/prefs/{key}`. An unset key returns `null`. Keys in use: `library.cols.<tab>`, `worklist.work`, `worklist.ncr`, `board.work`, `board.ncr`, `mywork`, `editor.mode`, and a BOM tree key per assembly.
- Every page component reads its route `params` once, wrapped in `untrack`. This is correct only because `App.svelte` remounts the page on every path change. If that `{#key}` is ever removed, those reads must become reactive.
- Banners (`.banner`, `.banner.info`, `.banner.warn`) are plain text blocks. Put buttons inline in the text.

### Verification

Done in the build session (2026-10-30):

- `go vet` clean; Go tests pass against PostgreSQL (`internal/app`, `internal/model`, `internal/repo`).
- Frontend type-check has no errors; production build succeeds.
- Seed data loaded on a local server matches TEST-PLAN.md Appendix A: next production number 100008-001, flat total of 100001-001 in 100004-201 is 10, traceability coverage, My Work order, mention notification.
- Access checks: locked DOC-0002 and SP-0002 pages return not found for `editor2`; locked BOM line hidden; Viewer edits refused; missing CSRF token refused; foreign Host refused (421) once Allowed addresses is set.
- CO-0002 approved with password signature and released: 100002-001 rev 02 Released, rev 01 Superseded, signed baseline tag, `git fsck --strict` passes.
- Backup, then restore of database, repository and attachments.
- Key screens load in headless Chromium on desktop and phone width with no browser errors.

Done for PR #4 (2026-10-31):

- `go vet` clean; all Go tests pass, including the new `TestPrefsUnsetIsNull`.
- `svelte-check`: 0 errors, 0 warnings.
- No frontend build and no browser check were run.

### Install state

- PR #4 merged into `main` on 2026-10-31; GitHub Actions published the new `ghcr.io/shwhat/designhub:latest`.
- The VM still runs the PR #2 image until Devon pulls.
- The Super Admin is created with `docker compose exec -it designhub designhub setup-admin`; the install does not prompt for an account.

### Next steps

1. Devon pulls the new image and runs the retest list above.
2. Get answers to the open bug questions (bugs 1, 3 and 4).
3. For bug 3, agree a visual direction with Devon before any redesign, using the frontend-design guidance.
4. Fix bugs on branches from `main`, open a pull request for Devon to merge, then Devon pulls the new image (`docker compose pull && docker compose up -d`).
5. Devon continues TEST-PLAN.md and adds failures to the bug list or as GitHub issues with label `v1-test`.

## Build environment notes

These apply to Claude Code cloud sessions.

### Tools

- Go 1.24, Node.js 22 and PostgreSQL 16 are available.
- proxy.golang.org is blocked. All Go dependencies, including golang.org/x modules, are vendored; always pass `-mod=vendor`.
- The npm registry is reachable. Docker is not available in the session, so images are built by GitHub Actions.
- Playwright browser downloads are blocked; the `@sparticuz/chromium` npm package provides a headless Chromium for screen checks.
- `setup-admin`, `seed` and `restore` require an interactive terminal.
- `@types/node` is not installed, so `web/vite.config.ts` is not type-checked.

### GitHub access

- Ask for push access to `Shwhat/DesignHub` before cloning if a branch will be pushed. Clone into `/home/claude/designhub`.
- `gh pr create` fails because GraphQL is blocked. Open pull requests with the REST API:
  `gh api repos/Shwhat/DesignHub/pulls -f base=main -f head=<branch> -f title="<title>" -F body=@<file>`
- The clone is shallow with a fetch refspec for `main` only. After pushing a new branch, add a refspec so the branch tracks its remote:
  `git config --add remote.origin.fetch '+refs/heads/<branch>:refs/remotes/origin/<branch>'`, then `git fetch origin` and `git config branch.<branch>.remote origin` and `git config branch.<branch>.merge refs/heads/<branch>`.

### Running the Go tests

```
pg_ctlcluster 16 main start
su postgres -c "psql -c \"CREATE ROLE dh LOGIN SUPERUSER PASSWORD 'dh'\""
su postgres -c "psql -c 'CREATE DATABASE dhtest OWNER dh'"
export DESIGNHUB_TEST_DATABASE_URL='postgres://dh:dh@127.0.0.1:5432/dhtest?sslmode=disable'
go vet -mod=vendor ./...
go test -mod=vendor ./internal/...
```

Without `DESIGNHUB_TEST_DATABASE_URL` the `internal/app` tests are skipped and still report `ok`. Check with `-run <TestName> -v` that a test actually ran.

### Frontend type-check

```
cd web && npm ci && npx svelte-check
```

### Build (only when Devon approves)

```
cd web && npm ci && npm run build && cd ..
CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags "-s -w" -o designhub ./cmd/designhub
```
