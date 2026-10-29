# DesignHub Handoff: Screen Layouts

Date: 2026-10-29
Purpose: continue DesignHub in a new conversation, starting the screen layouts.

## Start here

Paste this into the new chat:

> I'm continuing work on DesignHub. Read the handoff doc and the docs in GitHub repo Shwhat/DesignHub, branch docs/spec-v1 (PR #1): docs/SPEC.md and docs/DATA-MODEL.md. The data model is complete. We are now defining the v1 screen layouts, one screen at a time. Start by asking me the general layout questions in the handoff doc.

## Project rules

- Do not build, compile or package the app until Devon says so.
- Never use em dashes in any output.
- The app shows only DesignHub branding. Only exception: the Google sign-in button.
- Do not mention or include EPLAN.
- Answer questions simply, without extra unrequested actions.
- Deliverable documents contain no tips or meta-commentary.
- When reviewing a series of items, show progress as "N of M".

## Current state

| Item | State |
|---|---|
| Repo | github.com/Shwhat/DesignHub (private) |
| main | Go source of a single-user prototype. Will be reworked for the server version. |
| PR #1 (branch docs/spec-v1) | docs/SPEC.md v0.2 and docs/DATA-MODEL.md v0.1. Awaiting Devon's review. |
| Data model | Complete and fully reviewed. |
| Next | Screen layouts, then a v1 test plan. |

## Key decisions (detail in SPEC.md and DATA-MODEL.md)

**Platform**
- Server version only: Docker Compose on a Proxmox VM, local network first, domain later.
- Go backend, bundled frontend, no runtime dependence on the internet.
- Engineering data and pages in git as markdown. Work items, change order records, users, comments, notifications, lists and audit log in PostgreSQL. Attachments in a file store referenced from git. InfluxDB in v3.

**Identification**
- Production parts and assemblies: 6-digit base plus 3-digit category suffix (100042-001). Suffixes: -000 layout, -0XX and -1XX piece part, -2XX assembly, -3XX harness, -4XX PCB, -5XX PCBA, -6XX internal electronic component, -9XX interface or make-on-assembly.
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
- Engineering items have Owner and Assignee. Work items have Assignee and Reporter.

**Attachments**
- Tied to a revision, every file has a role, missing expected roles warn at release.
- Vendor download of native design files set per item.

## Screen layouts: questions to work through

**General layout (ask first)**
1. Navigation: left sidebar, top bar, or both?
2. Devices: desktop only for v1, or also tablet and phone for the factory floor?
3. Theme: light, dark, or both with a user setting?
4. Density: compact (more rows on screen) or comfortable?

**v1 screens, one at a time**
1. Sign-in
2. Home and My Work
3. Library browser (parts, assemblies, requirements, procedures, documents)
4. Engineering item view (fields, revisions, files, links, comments)
5. Assembly view (BOM, where-used, validation)
6. Item create and edit
7. Change order view, review and approval
8. Space home and page tree
9. Page view and the four editing modes
10. Work board and list views
11. Work item and NCR view
12. Search
13. Notifications inbox
14. Admin: users, roles, spaces, manufacturer list and other lists
15. User profile and settings
