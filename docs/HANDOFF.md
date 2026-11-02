# DesignHub Handoff

Date: 2026-11-01 (updated for version 2)
Purpose: everything needed to continue DesignHub in a new conversation. This document replaces all earlier handoff documents.

## Start here

Paste this into the new chat:

> I'm continuing work on DesignHub. Read docs/HANDOFF.md in GitHub repo Shwhat/DesignHub, branch main, then docs/SPEC.md, docs/DATA-MODEL.md, docs/SCREENS.md, docs/FRONTEND.md, docs/TEST-PLAN.md and docs/TEST-PLAN-V2.md. Version 1 is installed on my VM and testing is in progress. Version 2 is being specified: PR #8 (restyle) and PR #9 (version 2 documents) are open for review. The next steps are in the handoff.

## Documents

| Document | Contents | State |
|---|---|---|
| `docs/SPEC.md` | Product specification, decisions, v1/v2/v3 release plan | v0.4, v2 decisions added, in review (PR #9) |
| `docs/DATA-MODEL.md` | Identification, links, revisions, storage, field lists | v0.3, v2 records added, in review (PR #9) |
| `docs/SCREENS.md` | General layout, v1 screens and v2 screens (sections 18 to 27) | v0.3, in review (PR #9) |
| `docs/TEST-PLAN.md` | v1 test plan: checklist, phone and tablet checks, scenarios, seed data | v0.2, complete |
| `docs/TEST-PLAN-V2.md` | v2 test plan: checklist, regression, phone and tablet checks, scenarios SCN-10 to SCN-14, seed additions | v0.1, in review (PR #9) |
| `docs/FRONTEND.md` | Frontend framework and editor selection | v0.1, complete |
| `docs/GOOGLE-SIGNIN-SETUP.md` | Operator guide: Google Cloud OAuth client and DesignHub Google sign-in settings | v0.1, complete |
| `web/src/about/*.md` | In-app About guide: capabilities, rules, record fields, technical, release notes | Maintained with every pull request |
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
- Every pull request that changes what the app does, or the rules it follows, also updates the About guide in `web/src/about`: the affected pages and a new entry in `11-release-notes.md`. It also raises the version in `web/package.json` and `web/package-lock.json` (patch for fixes, minor for features). The version is shown in the guide and used as the image tag.

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
| 9. Fix the bug list, one branch per fix or group of fixes | PR #4 merged. Bug 3 in PR #8. Bugs 1 and 4 closed. |
| 10. Google sign-in, pulled forward from v2 | Complete (PR #6) |
| 11. In-app About guide | Complete (PR #7) |
| 12. Restyle to match OpenSchem (bug 3) | Done on branch `restyle-openschem`, PR #8 open for review |
| 13. Version 2 documents: data model, screens, test plan | Written on branch `docs/spec-v2`, PR #9 open for review |
| 14. Build version 2 on branch `v2-build`, one pull request at the end | Waits on PR #9 approval. Email built last, once the mail server is decided. |
| 15. Devon tests version 2 with TEST-PLAN-V2.md | Not started |

## Pull request history

| PR | Title | Merged |
|---|---|---|
| #1 | DesignHub specification v0.1 | 2026-10-29 |
| #2 | DesignHub version 1 | 2026-10-30 |
| #3 | Update handoff with install state and bug list | 2026-10-31 |
| #4 | Fix Library columns, phone search icon, banner spacing and svelte-check warnings | 2026-10-31 |
| #5 | Update handoff | 2026-10-31 |
| #6 | Google sign-in with allowed domains and addresses, signing PIN and recovery | 2026-10-31 |
| #7 | About guide inside the app, version 1.2.0 | 2026-11-01 |
| #8 | Restyle to match OpenSchem (bug 3), version 1.2.1 | Open |
| #9 | Version 2 data model, screens and test plan | Open |

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
| Domain name and HTTPS certificate (needed for Google sign-in). Cloudflare Tunnel with an Origin CA certificate is set up per the DesignHub Docker Quick Guide. | Devon |
| Google Cloud OAuth client for each installation (see Google sign-in setup below) | Devon |
| Mail server for email notifications (v2) | Devon |
| Real items for the realistic sample (TEST-PLAN.md Appendix B) | Devon |

## Bug list

Reported by Devon on 2026-10-30 after installing v1 on the VM. Findings are from reproducing on a local server with the same build.

| # | Report | Finding | State |
|---|---|---|---|
| 1 | Pages do not show up in the tree when created. A much more robust pages setup is needed. | Not reproduced. Creating a page with the sidebar "+" button (top level) and with "New child page" adds it to the tree immediately, on a fresh install and on the seed data; the server returns it in `GET /api/spaces/{id}/tree`. The sidebar "+" and "..." menu are the only ways to create a page. | Closed. Tree part resolved; the rest dropped by Devon 2026-11-01. |
| 2 | Parts do not show under Library. The item count goes up. | Cause: `GET /api/prefs/{key}` returned `{"ok":true}` when the user had no saved preference, so Library used that object as its column list and drew no columns. The work List view had the same fault. | Fixed in PR #4. Devon to retest. |
| 3 | The whole app looks very basic. | Design pass needed across all screens: typography, spacing, color, icons (text glyphs such as ▶ and ⋯ are still used in many places), headers, tables, cards and empty states. FRONTEND.md section 7 already names Lucide icons and the Inter font as the intended bundled assets. Direction agreed 2026-10-31: match OpenSchem's styling (program-design-guide.md) so the programs feel like one set: tokens, light and dark themes, typography, spacing, buttons, menus, dialogs, lists and icon style. Stay on Svelte and keep the current DesignHub layout; the OpenSchem ribbon, docking panels and plain-JavaScript structure are not adopted. | Done in PR #8: OpenSchem tokens and themes, system font, compact desktop sizing with touch-sized controls on phones, OpenSchem-style inline SVG icons (`web/src/lib/icons.ts`, `components/Icon.svelte`). Inter removed. Devon to review. |
| 4 | "where" (message cut off) | Dropped by Devon 2026-10-31 | Closed |

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

## Version 2

Scope (SPEC.md 16.2): Vendor and Contractor roles with company scoping and expiry; custom roles and editable workflows; sprints, milestones, timeline view and time tracking; templates, inline comments, PDF export; email notifications; CSV import and export.

Decisions made with Devon on 2026-11-01:

| Question | Decision |
|---|---|
| Order | Documents first, like v1, then build |
| Delivery | One branch `v2-build`, one pull request at the end |
| Restyle (bug 3) | Before v2, so v2 screens are built in the new style |
| Companies | Library record `COMP-NNNN` (basis for the v3 Supplier type) |
| Editable workflows | Task, issue, action, epic and NCR. Change orders stay fixed. One workflow per type for all spaces. |
| PDF export | Server side, headless Chromium in a new `pdf` container |
| CSV export | Every record type and every list |
| CSV import | Parts, BOM lines, requirements, work items, manufacturers, companies (to confirm in PR #9 review) |
| Mail server | To be decided. Email notifications are built last. |

Defaults proposed in the v2 documents for Devon's review (not yet confirmed):

| Topic | Proposal | Where |
|---|---|---|
| Vendor scope | Released revisions of items `supplied by` their company, released Drawing, Datasheet, 3D model, Schematic and BOM file files, NCRs with their company as Supplier, assigned actions | DATA-MODEL 5.5 |
| Supplier links | `supplied by` stored in PostgreSQL, changeable on released items without a change order | DATA-MODEL 3.1 |
| Internal comments | Any non-Vendor can mark a comment Internal; hidden from Vendors | DATA-MODEL 5.5 |
| Contractor scope | Edit pages and work items in member spaces; read released library items whose home space is a member space | DATA-MODEL 5.5 |
| Expiry | Required for Vendor and Contractor; Admin notices 14 days and 1 day before | DATA-MODEL 5.5 |
| Workflow categories | Open, Active, Done, Cancelled drive My Work, progress and reports | DATA-MODEL 4.7 |
| Milestone health | At risk within 7 days of target with under 80 percent done | DATA-MODEL 7.23 |
| Time entries | 0.25 h steps, up to 24 h, no future dates; not on epics | DATA-MODEL 7.24 |
| Templates | Stored in git under `templates/`; scope all spaces or one space | DATA-MODEL 7.25 |
| Dashboard view | Moved after v2 | SPEC 9.4 |

## Google sign-in (branch `google-signin`)

Decisions made with Devon on 2026-10-31:

| Question | Decision |
|---|---|
| When | Now, ahead of the v2 plan |
| Allowed domains | Set per installation in Admin, System. Nothing about a company is fixed in the code. |
| Individual addresses | Also allowed, for single-person companies using personal Google accounts |
| Google groups | Not used for now |
| Hostname | Set per installation (public address in Admin, System) |
| Role for new Google users | Viewer, no space access. An Admin assigns roles afterward. |
| Super Admin | Also signs in with Google once password sign-in is turned off |
| Recovery | `designhub recovery` on the host: new password and one password sign-in within one hour |
| Electronic signatures for Google users | Signing PIN set in Settings, Security. An Admin can clear a forgotten PIN. |

Implementation:

| Part | Where |
|---|---|
| Flow, settings API, account matching, recovery | `internal/app/google.go` (OpenID Connect code flow with PKCE, no new Go dependencies) |
| Signature check and signing PIN API | `internal/app/signing.go` |
| Password sign-in rules, shared session start | `internal/app/session.go` |
| Schema | `internal/db/migrations/0003_google_signin.sql` (`users.google_sub`, `users.signing_pin_hash`) |
| Recovery command | `cmd/designhub/main.go` |
| Screens | `web/src/pages/SignIn.svelte`, `web/src/pages/admin/Admin.svelte` (System card, user detail), `web/src/pages/Settings.svelte` (Security), `web/src/components/SignatureDialog.svelte` |
| Tests | `internal/app/google_test.go` (fake Google server: new account, domain and address rules, personal account with a company email refused, unverified and disabled refused, shared email refused, Super Admin linking, password off, recovery, signing PIN) |

API: `GET /api/auth/google/start`, `GET /api/auth/google/callback`, `GET` and `PUT /api/admin/google` (Super Admin), `PUT /api/me/signing-pin`, `clearSigningPin` on `PATCH /api/users/{id}`. `GET /api/signin` reports `google` and `passwordOff`.

Behavior notes:

- The ID token comes straight from Google's token endpoint over HTTPS, so its signature is not checked separately (Google's documented rule for this case). Issuer, audience, expiry and nonce are checked.
- Sign-in state is held in server memory for 10 minutes, so a server restart during a sign-in makes that attempt expire.
- The client secret is stored in the settings table, is never returned to the browser and is not written to the audit log.
- Turning off password sign-in requires the Super Admin making the change to have already signed in with Google.
- The rule "at least one active local Super Admin" is now "at least one active Super Admin".
- Viewer can read the Library. New Google users therefore see Library items until an Admin changes their role.

### Google sign-in setup (per installation)

1. In Google Cloud console, create a project, configure the OAuth consent screen (Internal for a single Workspace organization, External when personal addresses are allowed), and create an OAuth client of type Web application.
2. In DesignHub, Admin, System, Google sign-in: enter the public address (for example `https://designhub.example.com`) and save. Copy the redirect URI shown.
3. In the Google OAuth client, add that redirect URI under Authorized redirect URIs.
4. Paste the client ID and client secret into DesignHub, add allowed domains and addresses, save, and turn on "Show Google sign-in".
5. Set the Super Admin's email to their Google address, sign out, and sign in with Google.
6. Set a signing PIN in Settings, Security.
7. Turn on "Turn off password sign-in" when ready.
8. Make sure the public hostname is in Admin, System, Allowed addresses.

### Not yet verified

- No browser run of the new screens, because no frontend build has been approved.
- No sign-in against real Google.

## Build decisions (2026-10-30)

| Item | Decision |
|---|---|
| Base | Build from the specs on branch `v1-build`, created from `docs/spec-v1`. PR #1 is not waited on. |
| Git | Full git library: go-git v5 |
| Two-factor | Provided by Google sign-in (pulled into v1 on 2026-10-31) |
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
| Sign-in | Local accounts with Argon2id, and Google sign-in (OpenID Connect) limited to allowed domains and addresses. Signing PIN for Google users. |

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

1. Devon reviews and merges PR #8 (restyle), pulls the image and checks every screen in light and dark and on the phone.
2. Devon reviews PR #9 (version 2 documents): the proposed defaults in the Version 2 section above and the CSV import list.
3. After PR #9 is approved: build version 2 on `v2-build` from `main`, one pull request at the end. Email last.
4. Devon decides the mail server.
5. Devon continues TEST-PLAN.md and the retest list above, and adds failures to the bug list or as GitHub issues with label `v1-test`.

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
