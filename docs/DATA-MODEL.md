# DesignHub Data Model

| | |
|---|---|
| Document | DesignHub data model |
| Version | 0.1 (draft for review) |
| Date | 2026-10-29 |
| Owner | Devon |
| Status | Draft |
| Related | [SPEC.md](SPEC.md) |

## Contents

1. Scope
2. Identification
3. Links
4. Revisions and statuses
5. Library, spaces, people and permissions
6. Storage
7. Field lists
8. Placeholders for later versions

---

## 1. Scope

This document defines the records DesignHub stores, how they are identified, how they link, how they are revised and controlled, where they are stored, and the fields of every version 1 record type. Version 2 and version 3 features are included as placeholders only.

## 2. Identification

### 2.1 Principles

- Every record has a permanent identifier. It never changes and is never reused.
- An identifier is unique across all records of all types.
- Significance in a number is limited to the minimum needed for design, assembly and configuration management.
- Revision is not part of the identifier. It is shown alongside it, for example `100042-001 rev 02`.
- Every record also has a hidden internal ID (UUID) used by the database.

### 2.2 Production parts and assemblies

Format: 6-digit base number, dash, 3-digit suffix. Example: `100042-001`.

- The base number is non-significant and assigned automatically from a single counter.
- The suffix is semi-significant. The first digit gives the category; the second and third digits are sequential and used as the creator sees fit.
- A new item is assigned the next base number with suffix `-001`. The user may enter a different suffix following the rules below.

| Suffix | Category | Has BOM | Notes |
|---|---|---|---|
| -000 | Layout | No | Parts modeled from one layout share its base number |
| -0XX | Piece part | No | Single part, accompanied by a drawing or specification |
| -1XX | Piece part (spillover) | No | Same rules as -0XX |
| -2XX | Assembly | Yes | Two or more parts; drawing includes the BOM |
| -3XX | Harness | Yes | |
| -4XX | PCB | No | Includes flexible printed circuits |
| -5XX | PCBA | Yes | |
| -6XX | Internally developed electronic component | No | |
| -7XX | Reserved | | |
| -8XX | Reserved | | |
| -9XX | Interface or make-on-assembly part | No | Cannot be added as a BOM line |

### 2.3 Reference parts

Format: 7-digit non-significant number from its own counter. Example: `1000042`.

- Used for concept models, clearance checks, keep-out zones and other reference-only purposes.
- Never ordered or built.
- A reference part can be converted to a production part. Conversion issues a new production number and adds a `replaced by` link from the reference part to the new part.

The different base number lengths ensure production and reference numbers can never collide.

### 2.4 Purchased (COTS) parts

Format: manufacturer code, period, manufacturer order or catalog number.

| Example | Meaning |
|---|---|
| `PXC.3031212` | Phoenix Contact catalog number 3031212 |
| `SIE.3RT2015-1BB41` | Siemens order number 3RT2015-1BB41 |
| `RIT.8606000` | Rittal catalog number 8606000 |

- The manufacturer code comes from the manufacturer list (section 7.17).
- The item number is the unique key. The raw catalog number is also stored in the Order number field, and the descriptive type designation in the Type number field.
- Imported parts that already follow this format are kept as received.
- This format is used for every COTS part, so two manufacturers sharing a catalog number never collide.

### 2.5 M-State parts

Placeholder for a later version. Format: production part number with an M suffix, for example `100042-001-M01`. M00 is reserved for M-State layouts.

### 2.6 All other records

Format: prefix, dash, number. One counter per type. The number grows past 9999 when needed.

| Record | Prefix |
|---|---|
| Requirement | REQ |
| Procedure | PRC |
| Document | DOC |
| Page | PG |
| Change order | CO |
| Space | SP |
| Task | TSK |
| Issue | ISS |
| Nonconformance | NCR |
| Action | ACT |
| Epic | EPC |

### 2.7 Deletion and retirement

- Released records are never deleted. They are marked Obsolete or Cancelled.
- A draft that has never been released can be deleted. Its number is retired and never issued again.
- Only a Super Admin can permanently remove any other record. The removal is recorded in the audit log and the number is retired.
- A record's type never changes. A record created with the wrong type is retired and replaced, with a `replaced by` link.

## 3. Links

### 3.1 Link types

Links are typed and directional. A link is stored once, on the source record, and the reverse is shown automatically on the target.

| Link | From | To | Reverse |
|---|---|---|---|
| contains | Assembly | Part or assembly | used in |
| satisfies | Part, assembly | Requirement | satisfied by |
| verifies | Procedure | Requirement | verified by |
| derived from | Requirement | Requirement | derives |
| affects | Work item, change order | Engineering item | affected by |
| found on | Issue, NCR | Part or assembly (and serial number in v3) | has finding |
| fixed by | Issue, NCR | Change order | fixes |
| blocks | Work item | Work item | blocked by |
| duplicates | Work item | Work item | duplicated by |
| replaced by | Any record | Record of the same type | replaces |
| relates to | Any record | Any record | relates to |

`contains` is stored as BOM lines on the assembly (section 7.4).

### 3.2 Link rules

- A link points to the record in general and follows its current released revision.
- A link can be pinned to a specific revision, for example an NCR found on `100042-201 rev 02`.
- Links between records stored in git and records stored in PostgreSQL work the same way, by identifier.

## 4. Revisions and statuses

### 4.1 Revision scheme

- Revisions are numeric: 01, 02, 03.
- The first released revision is 01.
- A new record has no revision until it is first released.

### 4.2 Lifecycle

**Development.** A new item is created with its permanent number and status Draft. Edits are unrestricted. Every save is recorded with user, time and comment. No change order is needed.

**First release.** An item is released when something physical depends on it: it is ordered, built, tested, or handed to a vendor or the factory. Release is done through a change order and makes the item revision 01. Related items are released together on the same change order.

**After release.** A released revision is locked. When the owner is ready to change the item, they start the next revision (for example 02) as a draft. Starting a new revision does not require a change order. The new revision becomes released only when a change order approves and releases it. Until then, the previous revision remains the released revision.

### 4.3 Engineering item statuses

Status belongs to each revision.

| Status | Meaning | Editable |
|---|---|---|
| Draft | Working revision, not yet released | Yes |
| In Review | Included in a change order under review | Only through the change order |
| Released | The current approved revision | No |
| Superseded | An earlier released revision replaced by a newer one | No |
| Obsolete | The item is retired. No new builds or orders. | No |

Example: 01 Released, 02 Draft, 02 In Review, then 02 Released and 01 Superseded.

An item becomes Obsolete only through a change order.

### 4.4 Change order states

Draft, In Review, Approved, Released, Rejected, Cancelled.

### 4.5 Work item workflow

Tasks, issues, actions and epics: Open, In Progress, In Review, Done. Blocked and Cancelled are available from any open state.

### 4.6 Nonconformance workflow

| State | Entry requires |
|---|---|
| Reported | Required NCR fields |
| Dispositioned | Disposition and signed disposition approvals |
| Corrective Action | |
| Verified | Root cause and corrective action |
| Closed | Verification of effectiveness. A linked change order if a design change is required. |

## 5. Library, spaces, people and permissions

### 5.1 Library and spaces

| Area | Contains | Access controlled by |
|---|---|---|
| Library | Parts, assemblies, requirements, procedures, documents, change orders | Library role (company-wide) |
| Spaces | Pages, tasks, issues, NCRs, actions, epics | Space role (per space) |

- Every library record carries a home space. The home space is used for filtering and ownership. It does not limit access.
- Every page and work item belongs to exactly one space.
- Vendors and Contractors (v2) see only library records linked to their company or assigned projects.

### 5.2 Roles

- Each user holds one library role and may hold one role in each space.
- Roles control what a user can do. People fields on a record (section 5.3) record who is responsible and give no extra permissions.

### 5.3 People fields

| Field | On | Meaning |
|---|---|---|
| Created by | Every record | Set automatically, never changes |
| Owner | Engineering items, pages, change orders | Responsible person for the record over its life |
| Assignee | Engineering items, work items | Person currently working on it. Remains until changed. |
| Reporter | Work items | Person who raised it |
| Approvers | Change orders, NCR dispositions | People who must sign |
| Watchers | Every record | People who receive updates |

### 5.4 Record locking

- Any individual record can be locked to an access list of users.
- People not on the list do not see the record at all: not in search, lists, links, BOMs or notifications.
- BOM quantities, extended quantities and validation still include locked items, so totals remain correct.

## 6. Storage

### 6.1 Storage split

| Data | Storage |
|---|---|
| Parts, assemblies and BOMs | Git |
| Requirements | Git |
| Procedures | Git |
| Documents | Git |
| Pages | Git |
| Edits made under a change order | Git, on the change order's branch |
| Change order records (state, approvals, signatures) | PostgreSQL |
| Work items | PostgreSQL |
| Spaces, users, roles, memberships | PostgreSQL |
| Comments, watchers, notifications | PostgreSQL |
| Manufacturer list and other Admin lists | PostgreSQL |
| Audit log | PostgreSQL |
| Search index and link graph | PostgreSQL, rebuilt from git |
| Attachment files | File store, referenced from git |
| Time-series data | InfluxDB (v3) |

### 6.2 Change order branches

- Each change order has its own git branch. Edits to affected items are made on that branch.
- Release merges the branch into the main branch and creates a baseline tag.
- PostgreSQL stores the change order record, its state, approvals and signatures, and the branch and baseline names.

### 6.3 Attachments

- Every attachment belongs to a specific revision. A released revision's files never change.
- Files are not edited in place. Uploading a new version replaces the file in the draft revision only.
- All files, of any type, are kept in a file store on the server. Git holds a reference and checksum, tying each file to its revision.
- A file attached to several revisions is stored once.
- PDFs and images preview in the browser. Other files are download only.
- Native design project archives are stored as a single file per revision.
- Every attachment has a file role (section 7.19). A missing expected role produces a warning at release. It does not block release.
- Vendor download of native design files is controlled per item.

### 6.4 Synchronization

- The search index and link graph are rebuilt from git on startup and updated on every save.
- An Admin can trigger a full rebuild at any time.

## 7. Field lists

Field types: Text, Long text (markdown), Number, Date, Choice (from a fixed or Admin list), User, Link, Auto (set by the system), Computed (calculated on display).

### 7.1 Common engineering item fields

Used by parts, assemblies, requirements, procedures and documents.

| Field | Type | Required | Notes |
|---|---|---|---|
| Revision | Number | Auto | Blank until first release |
| Status | Choice | Auto | Section 4.3 |
| Owner | User | Yes | |
| Assignee | User | No | |
| Created by | User | Auto | |
| Home space | Space | Yes | |
| Labels | Text list | No | |
| Locked | User access list | No | Empty means not locked |
| Attachments | Files with role | No | |
| Links | Links | No | |

### 7.2 Part

All common engineering item fields, plus:

| Field | Type | Required | Notes |
|---|---|---|---|
| Item number | Text | Yes | Section 2. Never changes. |
| Part kind | Choice | Yes | Production, Reference, COTS. M-State placeholder. |
| Name | Text, max 50 | Yes | Shown in the drawing title block |
| Description | Long text | No | Searchable |
| Unit of measure | Choice | Yes | Admin list |
| Tracking | Choice | Yes | Serial, Lot, None. Default Lot. |
| Commodity code | Choice | No | Admin list |
| Vendors may download native files | Yes/No | Yes | Default No |
| Body | Long text | No | |

### 7.3 COTS part fields

Additional fields when Part kind is COTS:

| Field | Type | Required | Notes |
|---|---|---|---|
| Manufacturer | Choice | Yes | Manufacturer list |
| Order number | Text | Yes | Raw catalog number |
| Type number | Text | No | Descriptive type designation |
| Lifecycle status | Choice | Yes | Active, Not Recommended for New Design, Obsolete |
| Approved alternates | Links to COTS parts | No | |
| Unit cost | Money | No | |
| Lead time | Days | No | |
| Datasheet | Attachment, role Datasheet | No | |

### 7.4 Assembly and BOM

An assembly is a part with suffix -2XX, -3XX or -5XX and has all Part fields plus a BOM.

**BOM line**

| Field | Type | Required | Notes |
|---|---|---|---|
| Find number | Number | Yes | Auto-numbered in steps of 10, editable |
| Child item | Link to part or assembly | Yes | Follows the child's current released revision |
| Quantity | Number | Yes | Decimals allowed |
| Unit of measure | From child item | Auto | |
| Reference designators | Text list | No | Count must match quantity; mismatch is a warning |
| Line notes | Text | No | |
| Effectivity | Placeholder | No | v3 |

**BOM rules**

- The BOM is part of the assembly revision. Changing the BOM of a released assembly requires a new revision.
- A released assembly containing an unreleased child is a validation error.
- An assembly containing itself at any level is a validation error.
- -9XX parts cannot be added as BOM lines.

### 7.5 Requirement

All common engineering item fields, plus:

| Field | Type | Required | Notes |
|---|---|---|---|
| ID | REQ-NNNN | Auto | |
| Title | Text, max 100 | Yes | |
| Statement | Long text | Yes | Written as a "shall" statement |
| Rationale | Long text | No | |
| Category | Choice | Yes | Admin list |
| Source | Text | No | |
| Verification method | Choice, multiple | Yes | Inspection, Analysis, Demonstration, Test |
| Priority | Choice | Yes | Must, Should, Could |
| Coverage | Computed | Auto | Satisfied by and verified by, with gaps shown |

### 7.6 Procedure

All common engineering item fields, plus:

| Field | Type | Required | Notes |
|---|---|---|---|
| ID | PRC-NNNN | Auto | |
| Title | Text, max 100 | Yes | |
| Procedure type | Choice | Yes | Admin list |
| Purpose and scope | Long text | Yes | |
| Applies to | Links to parts or assemblies | No | |
| Tools and equipment | Text list | No | Links to Equipment records in v3 |
| Safety and prerequisites | Long text | No | |
| Estimated duration | Minutes | No | |
| Steps | Step list | Yes | |

**Step**

| Field | Type | Required | Notes |
|---|---|---|---|
| Step number | Number | Auto | Renumbers when steps move |
| Instruction | Long text | Yes | Images allowed |
| Acceptance criteria | Text | No | |
| Recorded value | Placeholder | No | v3: value type, units and limits |

### 7.7 Document

A document is a controlled file that does not belong to a single part. A part's drawing is an attachment on the part, not a document.

All common engineering item fields, plus:

| Field | Type | Required | Notes |
|---|---|---|---|
| ID | DOC-NNNN | Auto | |
| Title | Text, max 100 | Yes | |
| Document type | Choice | Yes | Admin list |
| Primary file | Attachment | Yes, at release | |
| Summary | Long text | No | Searchable |
| Applies to | Links to engineering items | No | |
| External reference | Text | No | For example a standard number |
| Vendors may download native files | Yes/No | Yes | Default No |

### 7.8 Page

| Field | Type | Required | Notes |
|---|---|---|---|
| ID | PG-NNNN | Auto | |
| Title | Text, max 150 | Yes | |
| Space | Space | Yes | |
| Parent page | Page | No | Empty means top level |
| Sort order | Number | Auto | Position among sibling pages |
| Page type | Choice | Yes | Admin list |
| Controlled | Choice | Yes | Use type default, Yes, No |
| Show released only | Yes/No | No | Controlled pages only |
| Body | Long text | No | May contain live blocks |
| Template | Template | No | |
| Revision, Status | | Auto | Controlled pages only. Uncontrolled pages keep version history without revisions. |
| Owner | User | Yes | |
| Created by, Labels, Locked, Attachments, Links | | | As section 7.1 |

### 7.9 Change order

| Field | Type | Required | Notes |
|---|---|---|---|
| ID | CO-NNNN | Auto | |
| Title | Text, max 100 | Yes | |
| Change type | Choice | Yes | Initial release, Design change, Documentation only, Obsolescence |
| Reason for change | Long text | Yes | |
| Description of change | Long text | Yes | |
| Affected items | Affected item list | Yes | |
| Existing stock disposition | Choice | No | Use as is, Rework, Scrap, Return to vendor |
| Need date | Date | No | |
| Priority | Choice | Yes | Low, Normal, High, Urgent |
| State | Choice | Auto | Section 4.4 |
| Owner | User | Yes | |
| Approvers | User list | Yes, before review | Decision, time, comment and electronic signature per approver |
| Released by | User | Auto | |
| Released on | Date and time | Auto | |
| Branch | Text | Auto | |
| Baseline | Tag | Auto | Created at release |
| Validation | Computed | Auto | Errors block approval; warnings do not |
| Created by, Home space, Labels, Locked, Attachments, Links | | | As section 7.1 |
| Effectivity | Placeholder | No | v3 |

**Affected item**

| Field | Type | Required | Notes |
|---|---|---|---|
| Item | Link to engineering item or controlled page | Yes | |
| Action | Choice | Yes | Release, Revise, Obsolete |
| From revision | Revision | Auto | Blank for initial release |
| To revision | Revision | Auto | The draft revision being released |

### 7.10 Space

| Field | Type | Required | Notes |
|---|---|---|---|
| ID | SP-NNNN | Auto | |
| Name | Text, max 60 | Yes | Can be renamed |
| Description | Text | No | |
| Home page | Page | Auto | Created with the space |
| Members | User and role list | Yes | At least one Admin |
| Controlled defaults | Page type and Yes/No list | Yes | |
| Archived | Yes/No | Auto | Read-only and hidden from navigation. Spaces are never deleted. |
| Created by | User | Auto | |
| Templates | Placeholder | No | v2 |

### 7.11 Work items (task, issue, action, epic)

**Common fields**

| Field | Type | Required | Notes |
|---|---|---|---|
| ID | TSK, ISS, ACT, EPC-NNNN | Auto | |
| Title | Text, max 150 | Yes | |
| Description | Long text | No | |
| Space | Space | Yes | |
| Status | Choice | Auto | Section 4.5 |
| Priority | Choice | Yes | Low, Normal, High, Urgent |
| Assignee | User | No | |
| Reporter | User | Auto | Defaults to creator, editable |
| Due date | Date | No | |
| Epic | Link to epic | No | Not on epics |
| Created by, Labels, Locked, Attachments, Links, Watchers | | | |
| Estimate, Time logged, Sprint, Milestone | Placeholders | No | v2 |

**Type-specific fields**

| Type | Field | Type | Required | Notes |
|---|---|---|---|---|
| Issue | Area | Choice | Yes | Admin list |
| Issue | Severity | Choice | Yes | Minor, Major, Critical |
| Action | Source | Link to page | No | |
| Epic | Start date | Date | No | |
| Epic | Target date | Date | No | |
| Epic | Progress | Computed | Auto | Share of child items Done |

### 7.12 Nonconformance (NCR)

All work item common fields, with Status following section 4.6, plus:

| Field | Type | Required | Notes |
|---|---|---|---|
| ID | NCR-NNNN | Auto | |
| Found on | Link to part or assembly, pinned to revision | Yes | |
| Serial or lot number | Text | No | Link to serial records in v3 |
| Quantity affected | Number | Yes | |
| Where found | Choice | Yes | Receiving inspection, In process, Final test, Field |
| Manufacturer | Choice | No | Manufacturer list |
| PO or work order number | Text | No | |
| Nonconformance description | Long text | Yes | |
| Requirement not met | Text or link | No | |
| Severity | Choice | Yes | Minor, Major, Critical |
| Containment | Long text | No | |
| Disposition | Choice | To reach Dispositioned | Use as is, Rework, Repair, Scrap, Return to vendor |
| Disposition approvers | User list with signatures | To reach Dispositioned | |
| Disposition notes | Long text | No | |
| Root cause | Long text | To reach Verified | |
| Corrective action | Long text | To reach Verified | |
| Verification of effectiveness | Long text | To reach Closed | |
| Requires design change | Yes/No | Yes | If Yes, a linked change order is required to close |

### 7.13 User

| Field | Type | Required | Notes |
|---|---|---|---|
| Username | Text | Yes | Unique |
| Display name | Text | Yes | |
| Email | Email | Yes | |
| Sign-in method | Choice | Yes | Local (v1), Google (v2) |
| Password | Hash | Local only | Argon2id |
| Two-factor | Yes/No | No | |
| Library role | Role | Yes | |
| Space roles | Space and role list | No | |
| Default editing mode | Choice | Yes | Visual, Split, Source, Preview toggle |
| Time zone | Choice | Yes | |
| Active | Yes/No | Yes | Users are never deleted |
| Last sign-in | Date and time | Auto | |
| Company, Account expiry, Email notifications | Placeholders | No | v2 |

### 7.14 Role

| Field | Type | Required | Notes |
|---|---|---|---|
| Name | Text | Yes | |
| Built-in | Yes/No | Auto | Built-in roles cannot be renamed or removed |
| Applies to | Choice | Yes | Library, Space, Both |
| Permissions | Permission list | Yes | |
| Can add or remove change order approvers | Yes/No | Yes | |
| Copied from | Placeholder | No | v2 |

Version 1 built-in roles: Super Admin, Admin, Release Authority, Approver, Editor, Contributor, Viewer.

### 7.15 Comment

| Field | Type | Required | Notes |
|---|---|---|---|
| Record | Link to any record | Yes | |
| Revision | Revision | Auto | Revision shown when the comment was made |
| Author | User | Auto | |
| Body | Long text | Yes | @mentions notify the mentioned user |
| Reply to | Comment | No | One level of replies |
| Created on | Date and time | Auto | |
| Edited on | Date and time | Auto | Earlier text kept in history |
| Resolved | Yes/No | No | |
| Inline anchor | Placeholder | No | v2 |

Comments are never deleted. The author or an Admin can hide a comment, and the hide is logged.

### 7.16 Notification

| Field | Type | Required | Notes |
|---|---|---|---|
| Recipient | User | Auto | |
| Event | Choice | Auto | Assigned, Mentioned, Status changed, Approval requested, Approval decided, Due soon, Overdue, Watched record changed |
| Record | Link to any record | Auto | |
| Triggered by | User | Auto | Blank for system events |
| Summary | Text | Auto | |
| Created on | Date and time | Auto | |
| Read | Yes/No | Auto | |
| Email sent | Placeholder | No | v2 |

Notifications are never sent about records the recipient cannot access.

### 7.17 Manufacturer

| Field | Type | Required | Notes |
|---|---|---|---|
| Code | Text, 2 to 5 capital letters | Yes | Unique. Cannot change once used in a part number. |
| Name | Text | Yes | |
| Website | Text | No | |
| Active | Yes/No | Yes | Inactive codes cannot be used for new parts |

Initial list:

| Code | Manufacturer |
|---|---|
| PXC | Phoenix Contact |
| SIE | Siemens |
| RIT | Rittal |
| WAG | WAGO |
| SCH | Schneider Electric |
| ABB | ABB |
| AB | Allen-Bradley (Rockwell Automation) |

### 7.18 Admin lists

| Field | Type | Required | Notes |
|---|---|---|---|
| Value | Text | Yes | |
| Sort order | Number | Yes | |
| Active | Yes/No | Yes | Values in use are deactivated, never deleted |

| List | Initial values |
|---|---|
| Units of measure | Each, meter, foot, kilogram, liter |
| Commodity codes | None |
| Requirement categories | Functional, Performance, Interface, Safety, Regulatory, Environmental |
| Procedure types | Test, Inspection, Work instruction, Calibration, Maintenance |
| Document types | Specification, Standard, Drawing, Interface control, Plan, Report, Agreement, Other |
| Page types | General, Design review, Test report, Meeting notes, Decision record |
| Issue areas | Design, Factory, Process, Supplier |

### 7.19 File roles

Admin list with one additional field.

| Field | Type | Required | Notes |
|---|---|---|---|
| Value | Text | Yes | |
| Expected on suffixes | Suffix list | No | A release warning is shown when an item with a listed suffix has no file with this role |
| Sort order | Number | Yes | |
| Active | Yes/No | Yes | |

| Role | Expected on suffixes |
|---|---|
| Drawing | -0XX, -1XX, -2XX, -3XX, -4XX, -5XX |
| 3D model | None |
| Schematic | -3XX, -4XX, -5XX |
| Datasheet | None |
| Native design files | None |
| Image | None |
| Other | None |

## 8. Placeholders for later versions

| Placeholder | Version | Location |
|---|---|---|
| M-State parts | Later | Section 2.5 |
| Serial numbers and serial records | v3 | Part Tracking field, NCR, Found on link |
| Effectivity | v3 | BOM line, change order |
| Procedure recorded values | v3 | Procedure step |
| Equipment links | v3 | Procedure tools and equipment |
| Supplier records | v3 | COTS part, NCR |
| Estimate, time logged, sprint, milestone | v2 | Work items |
| Templates | v2 | Space |
| Inline comments | v2 | Comment |
| Company and account expiry | v2 | User |
| Email notifications | v2 | User, notification |
| Custom roles | v2 | Role |
| Google sign-in | v2 | User |
