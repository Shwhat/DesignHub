# DesignHub Handoff: Data Model

Date: 2026-10-29
Purpose: continue defining the DesignHub data model in a new conversation.

## Start here

Paste this into the new chat:

> I'm continuing work on DesignHub. Read the handoff doc and the spec (docs/SPEC.md in GitHub repo Shwhat/DesignHub, PR #1). We are defining the data model, one topic at a time. Topic 1 is drafted and has 4 open decisions. Start by asking me those 4 decisions.

## Project rules

- Do not build, compile or package the app until Devon says so.
- Never use em dashes in any output.
- The app shows only DesignHub branding. Only exception: the Google sign-in button.
- Do not mention or include EPLAN.
- Answer questions simply, without extra unrequested actions.
- Deliverable documents contain no tips or meta-commentary.

## Current state

| Item | State |
|---|---|
| Repo | github.com/Shwhat/DesignHub (private) |
| main | Go source of a single-user prototype (markdown items, embedded git, BOM, traceability, validation). Will be reworked for the server version. |
| PR #1 | docs/SPEC.md (v0.2) and docs/DATA-MODEL.md (v0.1). Awaiting Devon's review. |
| Data model | Complete. All five topics decided and recorded in DATA-MODEL.md. |
| Next | Screen layouts, then a v1 test plan. |

## Key decisions already made (detail in SPEC.md)

- Server version only: Docker Compose on a Proxmox VM, local network testing first, domain later.
- Go backend, bundled frontend, no runtime dependence on the internet. Online services allowed but optional.
- Storage split: controlled engineering data (requirements, parts, assemblies, BOM, procedures, documents, pages, change orders) as markdown in git. Work items, comments, notifications, users, roles, audit log and search index in PostgreSQL. InfluxDB for time-series, read and write.
- Devon is new to PostgreSQL and InfluxDB; setup must be automatic.
- Many users editing from day one.
- Roles: Super Admin, Admin, Release Authority, Approver, Editor, Contributor, Contractor, Vendor, Viewer. Assigned per space. Approvers on change orders can be added or removed based on role.
- Pages: spaces with renameable names and permanent IDs; first space "Factory Buildout"; four user-selectable editing modes (Visual, Split, Source, Preview toggle); every page type can be set controlled or not.
- Work: tasks, issues, NCRs, actions, epics; sprints and milestones; time tracking; NCRs from the start.
- Priority: product design first, factory second, both important.

## Data model topics

All complete. Decisions are recorded in [DATA-MODEL.md](DATA-MODEL.md).

1. Identification and links
2. Revisions and statuses
3. Library, spaces, people and permissions
4. Storage boundary between git, PostgreSQL and the file store
5. Field lists for v1 records, with placeholders for v2 and v3
