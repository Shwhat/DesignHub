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
| PR #1 | docs/SPEC.md, specification v0.1 with all decisions and the v1/v2/v3 release plan. Awaiting Devon's review. |
| Next | Define the data model, then screen layouts, then a v1 test plan. |

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

1. ID scheme and how records link (drafted, decisions open)
2. How revisions and statuses work
3. How spaces, ownership and permissions attach to every record
4. The boundary between git and PostgreSQL, and how they reference each other
5. Full field list for v1 items

v2 and v3 items need placeholders only, so the structure has room for them.

## Topic 1 draft: IDs and links

**IDs**
- Format PREFIX-NUMBER (PRT-0042, NCR-0007, CO-0003). One counter per type, grows past 9999.
- Permanent: never changes, never reused. Type cannot change; wrong type is obsoleted and replaced.
- Revision is not part of the ID (PRT-0042 rev B).
- Every record also has a hidden internal ID (UUID) used by the database.

**Links**
- Typed and directional, stored once on the source record, reverse shown automatically.

| Link | From → To | Reverse |
|---|---|---|
| contains | Assembly → Part or Assembly | used in |
| satisfies | Part, Assembly → Requirement | satisfied by |
| verifies | Procedure → Requirement | verified by |
| affects | Work item, Change order → engineering item | affected by |
| fixed by | Issue, NCR → Change order | fixes |
| blocks | Work item → Work item | blocked by |
| replaced by | any → same type | replaces |
| relates to | any → any | relates to |
| duplicates | Work item → Work item | duplicated by |

- Links point to the item generally; can be pinned to a revision (for example an NCR found on ASM-0001 rev B).
- Links between PostgreSQL and git records work the same way, by ID.
- No deleting: records are marked Obsolete or Cancelled. Only a Super Admin can permanently remove a record, and it is logged.

**Open decisions**
1. Plain sequential IDs, or meaningful part numbers (for example 100-xxxx electrical)? Recommendation: plain IDs plus an optional "Company part number" field.
2. Company prefix on IDs (SA-PRT-0042) or short (PRT-0042)?
3. Is the no-delete rule acceptable?
4. Any link types missing?

## Topics 2 to 5: questions to work through

**2. Revisions and statuses**
- Revision scheme: letters (A, B, C), numbers, or letters for released and numbers for drafts (A, A.1, B)?
- What triggers a new revision; does every released change bump it?
- Status sets per type (engineering items vs work items vs change orders) and allowed transitions.
- What can be edited in each status.

**3. Spaces, ownership, permissions**
- Does every record belong to exactly one space?
- Owner vs assignee vs reporter.
- How vendor and contractor scoping attaches (company and project fields).
- Permission checks per record vs per space.

**4. Git and PostgreSQL boundary**
- Exact list of what lives where (draft in SPEC.md 4.2).
- How the search index and link graph stay in sync with git.
- How change order branches map to git branches and what PostgreSQL stores about them.
- Attachments and large files.

**5. v1 field lists**
- Full fields, types and required/optional for: requirement, part, assembly, procedure, document, page, change order, space, task, issue, NCR, action, epic, user, role, comment, notification.
