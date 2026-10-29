# DesignHub

Local-only engineering wiki, issue tracker and bill of design, delivered as a single portable executable.

- Items (requirements, parts, assemblies, procedures, issues, pages) are markdown files with YAML frontmatter.
- Every save is committed to a standard git repository inside the data folder.
- Multi-level bill of design with extended quantities, flattened parts lists and where-used.
- Requirement traceability (satisfied by, verified by) and a validator for broken links, BOM cycles and release rules.
- Binds to 127.0.0.1 only and makes no outbound network calls.

## Build

Requires Go 1.22 or newer. All dependencies are vendored, so no internet access is needed.

Windows (from cmd):

```
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -mod=vendor -trimpath -ldflags "-s -w" -o DesignHub.exe .
```

## Run

```
DesignHub.exe [-data <folder>] [-port <number>] [-no-browser]
```

A `data` folder is created next to the executable on first run, seeded with a small demo data set.

## Item format

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
Body in markdown. Reference another item with [[PRT-0001]].
```

Embed a live bill of design in any page with a fenced block whose info string is `bom ASM-0001`.

| Type | ID prefix |
|---|---|
| requirement | REQ |
| part | PRT |
| assembly | ASM |
| procedure | PRC |
| issue | ISS |
| page | PG |

Statuses: Draft, In Review, Released, Obsolete.

## Source layout

| File | Purpose |
|---|---|
| `main.go` | HTTP server, pages and handlers |
| `store.go` | Item loading, BOM rollup, links and validation |
| `git.go` | Minimal git implementation (standard library only) |
| `vendor/` | Vendored dependencies (goldmark, yaml.v3) |
