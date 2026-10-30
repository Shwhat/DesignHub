# DesignHub Handoff

Date: 2026-10-30
Purpose: everything needed to continue DesignHub in a new conversation. This document replaces all earlier handoff documents.

## Start here

Paste this into the new chat:

> I'm continuing work on DesignHub. Read docs/HANDOFF.md in GitHub repo Shwhat/DesignHub, branch docs/spec-v1 (PR #1), then docs/SPEC.md, docs/DATA-MODEL.md, docs/SCREENS.md and docs/TEST-PLAN.md. The spec, data model, screen layouts and v1 test plan are complete. The next step is the Proxmox host preparation instructions.

## Documents

| Document | Contents | State |
|---|---|---|
| `docs/SPEC.md` | Product specification, decisions, v1/v2/v3 release plan | v0.2, complete |
| `docs/DATA-MODEL.md` | Identification, links, revisions, storage, field lists | v0.1, complete |
| `docs/SCREENS.md` | General layout and the 15 v1 screens | v0.1, complete |
| `docs/TEST-PLAN.md` | v1 test plan: checklist, phone and tablet checks, scenarios, seed data | v0.1, complete |
| `docs/HANDOFF.md` | This document: rules, state, next steps, technical notes | Current |

All documents are on branch `docs/spec-v1` in PR #1, awaiting Devon's review.

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
| 5. Proxmox host preparation instructions for Devon (VM, fixed IP; Docker is already installed on the VM) | Next |
| 6. Build v1, feature by feature on branches, only when Devon says to build | To do |
| Devon reviews and merges PR #1 | Any time |

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
| Review and merge PR #1 | Devon |
| Two-factor setup flow (SCREENS.md section 17) | To be defined |
| Meaning of "major update" for the Home layout reset (SCREENS.md section 17) | To be defined |
| Git implementation: extend `git.go` or adopt a full git library (branches, merges and tags are needed for change orders) | Decide before build |
| Domain name and HTTPS certificate (needed for Google sign-in) | Devon |
| Mail server for email notifications (v2) | Devon |
| Session inactivity timeout value (TEST-PLAN.md SGN-07) | To be defined |
| Status of affected items after a change order is rejected or cancelled | To be defined |
| Whether every assigned approver must approve a change order | To be defined |
| Real items for the realistic sample (TEST-PLAN.md Appendix B) | Devon |

## Target architecture

| Part | Choice |
|---|---|
| Backend | Go, single binary, JSON API plus server-rendered or bundled web UI |
| Frontend | Bundled assets only, no CDN. Markdown editor supporting Visual, Split, Source and Preview toggle modes, all writing the same markdown |
| Engineering data | Markdown files with YAML frontmatter in a git repository on a Docker volume |
| App data | PostgreSQL |
| Time-series | InfluxDB (v3 of the release plan), read and write |
| Packaging | Docker Compose: designhub, postgres, influxdb, proxy (HTTPS) |
| Host | Proxmox VM, local network first, company domain later |
| Sign-in | Local accounts with Argon2id and optional TOTP in v1; Google sign-in in v2 once a domain with HTTPS exists |

Setup must create and connect the databases automatically. Devon is new to PostgreSQL and InfluxDB.

## Current code on main (prototype)

A single-user prototype written in Go 1.24 with the Go standard library plus two vendored libraries. It proves the core ideas and is the starting point for the server version.

| File | Contents |
|---|---|
| `main.go` | HTTP server on 127.0.0.1, server-rendered HTML pages, handlers, line diff, browser launch |
| `store.go` | Item parsing (YAML frontmatter), loading, links, backlinks, BOM rollup, validation, ID allocation, seed data |
| `git.go` | Minimal git implementation using only the standard library |
| `vendor/` | goldmark v1.7.4 (markdown), gopkg.in/yaml.v3 v3.0.1 |

### Prototype features

- Item types: requirement (REQ), part (PRT), assembly (ASM), procedure (PRC), issue (ISS), page (PG). These IDs and the letter revisions predate the data model and will be replaced.
- Statuses: Draft, In Review, Released, Obsolete.
- Item file format:

```
---
id: ASM-0001
type: assembly
title: Motor control panel
rev: A
status: Draft
owner: devon
links:
  satisfies: [REQ-0001]
bom:
  - {part: PRT-0001, qty: 2, ref: K1 K2}
---
Markdown body. [[PRT-0001]] links to an item.
```

- A fenced block with info string `bom ASM-0001` embeds a live Bill of Design table in a page.
- Multi-level BOM with extended quantities, flattened parts list, where-used.
- Traceability view (satisfies, verifies) with coverage status.
- Validator: missing fields, unknown type or status, ID prefix mismatch, broken links, BOM cycles, non-numeric quantities, Released assembly containing non-released items.
- Browser editor (raw text area) with change note. Every save is a git commit attributed to the OS user.
- Edit conflict detection: the editor carries a hash of the file as opened; a save is refused with HTTP 409 if the file changed since.
- History per item with a line-by-line diff of each version.
- Full-text search across ID, title and body.
- Security: binds to 127.0.0.1 only, rejects requests whose Host is not 127.0.0.1 or localhost, rejects cross-origin POSTs, strict Content-Security-Policy, markdown rendered with raw HTML disabled.

### git.go details

- Writes loose objects (zlib), trees, commits, the branch ref and a version 2 index. Plain git reports a clean status and `git fsck --strict` passes on the result.
- Each commit snapshots every file in the data folder except `.git`.
- Commits are never signed.
- File history walks first parents from HEAD and compares blob IDs per path.
- Reads loose objects only. If another git tool packs the repository, older history is reported as unavailable rather than failing.
- No branches, merges or tags yet. Change orders need these.

### What must change for the server version

- Remove the 127.0.0.1 binding and the Host header restriction; bind to all interfaces inside the container behind the proxy, with an allowed-hosts setting.
- Remove browser launch and the console prompts.
- Replace OS user attribution with signed-in DesignHub users.
- Add PostgreSQL, accounts, roles, sessions and CSRF protection.
- Replace the raw text area with the bundled multi-mode editor.
- Update README: the prototype README states no outbound network calls. The product rule is now "no dependence on the internet; optional online services allowed."
- Data folder becomes a Docker volume.

## Build environment notes

These apply to Claude Code cloud sessions used so far:

- Go 1.24 is preinstalled.
- proxy.golang.org is blocked (HTTP 403). Fetching modules with `GOPROXY=direct GOSUMDB=off` works for modules hosted on GitHub.
- gopkg.in is blocked. Workaround used: clone the GitHub source repo and use a `replace` directive, then vendor. The replace was removed before committing; the vendored copy is what builds.
- golang.org/x modules are likely blocked, which is why go-git was not used (it depends on them). Check this again before choosing a git library or any dependency.
- Vendor all dependencies (`go mod vendor`) so the repo builds offline.
- npm registry is reachable, so frontend packages can be fetched and bundled.
- Build command (only when Devon says to build):

```
CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags "-s -w" -o designhub .
```

## Prototype acceptance checks

These checks passed on the prototype and should keep passing, adapted, in v1:

- Every page returns 200; unknown item returns 404.
- Page with an embedded BOM shows the BOM table and flattened parts.
- Extended quantity is correct for a multi-quantity line.
- Where-used on a part lists its parent assembly.
- History shows the initial commit and each later change with its note and author.
- Traceability flags a requirement with no verification as "No test" and shows complete coverage correctly.
- Seed data validates with no errors.
- Creating an item allocates the next ID and opens the editor.
- Validator reports a broken link.
- History diff shows added and removed lines.
- A stale save is rejected with 409 and the first save is kept.
- Changing an item's ID in the editor is rejected.
- A file without frontmatter is rejected.
- Release rule: a Released assembly containing a Draft part is an error; with all children Released, no error.
- A BOM cycle is detected without hanging.
- Search finds text in item bodies.
- `git log`, `git status` (clean) and `git fsck --strict` succeed on the data folder.
- A request with a foreign Host header is refused.
