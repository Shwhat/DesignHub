# DesignHub Specification

| | |
|---|---|
| Document | DesignHub product specification |
| Version | 0.2 (draft for review) |
| Date | 2026-10-29 |
| Owner | Devon |
| Status | Draft |

## Contents

1. Purpose
2. Guiding principles
3. Deployment
4. Architecture and data storage
5. Users, sign-in and roles
6. Spaces and Pages
7. Engineering data and the Bill of Design
8. Change control
9. Work
10. Notifications
11. Search
12. Time-series data
13. Optional online services
14. Security and audit
15. Backup and recovery
16. Release plan
17. Open items

---

## 1. Purpose

DesignHub is a self-hosted system that combines three functions in one connected application:

- **Pages:** documentation organized in spaces.
- **Work:** task, issue and nonconformance tracking with boards, sprints and milestones.
- **Bill of Design:** controlled engineering data (requirements, parts, assemblies, procedures) with revision control, change orders and released baselines.

Every record in DesignHub can link to every other record. A requirement links to the design that satisfies it, the design links to its parts, a nonconformance links to the affected part and serial number, and a page can display any of this live.

Initial use is product design, followed by the factory build-out. Both are first-class use cases.

## 2. Guiding principles

1. **No dependence on the internet.** DesignHub runs fully on the local network. It may use online services when available, but every such feature is optional, can be switched off, and fails without blocking the application.
2. **Portable engineering data.** Controlled engineering data is stored as plain markdown files in a standard git repository, readable without DesignHub.
3. **Everything is traceable.** Every change records who made it, when and why.
4. **Own branding only.** The application displays only DesignHub branding. The single exception is the Google sign-in button, whose wording and logo are required by Google.
5. **Bundled assets.** All fonts, scripts and styles ship with the application. Nothing is loaded from external servers at runtime.

## 3. Deployment

### 3.1 Target environment

| Item | Value |
|---|---|
| Host | Proxmox |
| Guest | Linux virtual machine |
| Packaging | Docker Compose |
| Initial network | Local network, fixed IP address |
| Later network | Company domain name with HTTPS |

### 3.2 Containers

| Container | Purpose |
|---|---|
| `designhub` | Application server and web interface |
| `postgres` | Application database |
| `influxdb` | Time-series database (enabled in a later release) |
| `proxy` | HTTPS termination in front of the application |

Setup creates, configures and connects the databases automatically. The whole system starts and stops with a single command.

### 3.3 Persistent volumes

| Volume | Contents |
|---|---|
| `repo` | Engineering data git repository and attachments |
| `pgdata` | PostgreSQL data |
| `influxdata` | InfluxDB data |
| `backups` | Scheduled backup output |

### 3.4 Updates

A new version is deployed by pulling a new application image and restarting the stack. Database schema changes are applied automatically on startup and recorded in the audit log.

## 4. Architecture and data storage

### 4.1 Components

- **Application server:** written in Go. Serves the web interface and a JSON API used by the interface.
- **Web interface:** runs in the browser. Includes a bundled markdown editor supporting all editing modes in section 6.3.
- **Engineering repository:** a git repository on the `repo` volume.
- **PostgreSQL:** application and activity data.
- **InfluxDB:** time-series data.

### 4.2 Storage split

Controlled engineering data is stored in git. Day-to-day activity and application data is stored in PostgreSQL.

| Data | Storage | Reason |
|---|---|---|
| Requirements, parts, assemblies, Bill of Design | Git | Revision controlled and released through change orders |
| Procedures, work instructions | Git | Revision controlled |
| Pages | Git | Versioned and portable |
| Edits made under a change order | Git, on the change order's branch | Reviewed and released as one set |
| Change order records (state, approvals, signatures) | PostgreSQL | Workflow and signature data |
| Attachments | File store, referenced from git by checksum | Tied to the revision they belong to |
| Work items (tasks, issues, NCRs, actions, epics) | PostgreSQL | Frequent updates by many concurrent users |
| Sprints, milestones, time entries | PostgreSQL | Activity data |
| Comments, mentions, watchers, notifications | PostgreSQL | High volume |
| Users, roles, permissions, sessions | PostgreSQL | Application data |
| Manufacturer list and other Admin lists | PostgreSQL | Lookup lists |
| Audit log | PostgreSQL | Append-only record |
| Search index and link graph | PostgreSQL | Derived from git, rebuildable at any time |

### 4.3 How the two stores connect

- Every record has a permanent ID (for example `100042-001`, `PXC.3031212`, `NCR-0007`). Links between records always use IDs. Identification, links, revisions and fields are defined in [DATA-MODEL.md](DATA-MODEL.md).
- Work items reference engineering items by ID. The engineering item's page shows the linked work items.
- The search index and link graph are rebuilt from git on startup and updated on every save.

### 4.4 Concurrent editing

- Saves to engineering data and pages use conflict detection. If an item changed after a user opened it, the save is refused and the user is shown the differences to resolve. One user's save never overwrites another's.
- Work item updates are handled by PostgreSQL transactions and appear to other users without reloading the page.

### 4.5 Data export

Work items, change order records, comments and time entries can be exported to markdown and CSV on demand and on a schedule, so no data is held only in the database.

## 5. Users, sign-in and roles

### 5.1 Sign-in

| Method | Availability |
|---|---|
| Local accounts (username and password) | Always available |
| Google sign-in | Available once DesignHub has a fixed HTTPS address |

- Passwords are hashed with Argon2id.
- Optional two-factor authentication with an authenticator app (TOTP).
- Google sign-in is restricted to the company's Google Workspace domain and, optionally, to specific Google groups.
- An account is created automatically on first Google sign-in with a default role set by an Admin.
- A disabled Google account can no longer sign in.
- At least one local Super Admin account always exists, so an outage of Google or the internet cannot lock out administrators.

### 5.2 Roles

| Role | Permissions |
|---|---|
| **Super Admin** | All permissions. System settings, sign-in configuration, database connections, backups. Creates and removes Admins. Cannot be locked out. |
| **Admin** | Manages users and roles (except Super Admins), workflows, templates, spaces and project settings. |
| **Release Authority** | Gives final sign-off that releases a change order and creates a baseline. |
| **Approver** | Approves or rejects change orders and NCR dispositions assigned to them. |
| **Editor** | Creates and edits engineering items, pages and work items. Opens change orders. |
| **Contributor** | Creates and updates work items and NCRs, logs time and test data. Read-only on engineering data. |
| **Contractor** | Edits only within assigned projects or spaces. No access elsewhere. Access can expire on a set date. |
| **Vendor** | Sees only parts, drawings and NCRs linked to their own company. Can comment, upload documents and respond to NCRs and corrective action requests. |
| **Viewer** | Read-only access to assigned spaces. |

### 5.3 Role rules

- Each user holds one library role, which controls engineering items and change orders company-wide, and may hold one role in each space, which controls that space's pages and work items.
- Individual records can be locked to an access list. Users not on the list do not see the record.
- Vendors and Contractors are linked to a company record and see only records linked to that company or their assigned projects.
- Accounts for Vendors and Contractors can have an expiry date.
- Admins can create custom roles by copying an existing role and adjusting its permissions.
- Admins define which roles may add or remove approvers on a change order.
- Every permission change is recorded in the audit log.

## 6. Spaces and Pages

### 6.1 Spaces

- A space is a top-level area with its own permissions, home page and page tree.
- Pages and work items belong to exactly one space. Engineering items and change orders live in a company-wide library and carry a home space for filtering and ownership.
- Each space has a permanent internal ID (for example `SP-0001`) and a display name. Renaming a space changes only the display name; links, permissions and storage use the ID.
- Initial space: **Factory Buildout**.

### 6.2 Page structure

- Pages nest under parent pages to form a tree.
- Navigation sidebar showing the page tree, and breadcrumbs at the top of each page.
- Each space home page shows recent changes and pinned links.

### 6.3 Editing modes

Each user selects a default editing mode in their profile and can switch modes on any page. All modes read and write the same markdown.

| Mode | Description |
|---|---|
| Visual | Word-processor style editing. Formatting is applied as the user types; markdown is not shown. |
| Split | Markdown on the left, live preview on the right. |
| Source | Markdown only, full width. |
| Preview toggle | Markdown with a button to switch to the rendered view. |

All modes include a formatting toolbar, image and file paste or drag and drop, and autosaved drafts.

### 6.4 Live content blocks

Pages can embed blocks that always display current data:

| Block | Shows |
|---|---|
| Bill of Design | Multi-level BOM for an assembly with extended quantities |
| Work list | Work items matching a filter, for example open NCRs on an assembly |
| Traceability | Requirements with their satisfying design and verifying procedures |
| Change orders | Open or recent change orders for a space or item |
| Chart | Time-series data from InfluxDB |
| Item card | Status and revision of a linked item |

### 6.5 Templates

- Templates for design review, test report, meeting notes, work instruction, procedure and decision record.
- Admins create and edit templates per space.

### 6.6 Collaboration

- Page comments and inline comments on selected text.
- @mentions notify the mentioned user.
- Users can watch a page to be notified of changes.

### 6.7 Controlled pages

- Every page type has a Controlled setting. Admins set the default per type, and individual pages can override it.
- A controlled page changes only through a change order.
- Changing a page from controlled to uncontrolled requires a change order.
- A controlled page can be pinned to its released revision so readers always see the approved version.

### 6.8 History

- Full version history for every page, with a line-by-line comparison between any two versions.
- Any earlier version can be restored. A restore is recorded as a new version.

### 6.9 Output

- Export a page or page tree to PDF.
- Print-friendly view.

## 7. Engineering data and the Bill of Design

### 7.1 Item types

| Type | Identifier | Purpose |
|---|---|---|
| Requirement | REQ | A requirement the design must meet |
| Part | `100042-001` (production), `1000042` (reference), `PXC.3031212` (purchased) | A purchased or manufactured component |
| Assembly | `100042-201` (suffix -2XX, -3XX harness, -5XX PCBA) | An item made from parts and sub-assemblies |
| Procedure | PRC | Test, inspection or work procedure |
| Document | DOC | Drawing, specification or other controlled document |
| Supplier | SUP | A company that supplies parts or services |
| Equipment | EQP | Factory equipment and tooling |
| Location | LOC | A physical location (building, area, cell, station) |
| Page | PG | Documentation page |
| Change order | CO | A controlled change to engineering data |

### 7.2 Common fields

ID, type, title, revision, status, owner, assignee, home space, labels, links, description (markdown body), attachments.

Revisions are numeric (01, 02, 03). A new item has no revision until it is first released.

Statuses: Draft, In Review, Released, Superseded, Obsolete.

Full field lists are in [DATA-MODEL.md](DATA-MODEL.md).

### 7.3 Part attributes

Part kind (production, reference, purchased), unit of measure, tracking (serial, lot or none), commodity code. Purchased parts add manufacturer (from the manufacturer list), order number, type number, approved alternates, unit cost, lead time and lifecycle status (Active, Not Recommended for New Design, Obsolete).

### 7.4 Bill of Design

- Multi-level structure with quantity and reference designators per line.
- Extended quantities and a flattened parts list.
- Where-used for every part and assembly.
- Impact analysis: everything affected by a change to or obsolescence of an item.
- Effectivity: a BOM line or revision can apply from a serial number or date onward.
- Validation: missing items, cycles, and released assemblies containing unreleased items are reported as errors.

### 7.5 Traceability

- Relation types: satisfies (design to requirement), verifies (procedure to requirement), derived from (requirement to requirement), affects, found on, fixed by, relates to.
- Traceability view showing each requirement's design and verification coverage.

### 7.6 Attachments

- Files of any type, attached to a specific revision. A released revision's files never change.
- Files are kept in a file store on the server and referenced from git by checksum, so the repository stays fast.
- Each file has a role (for example Drawing or Schematic). A missing expected role produces a warning at release.
- Vendor download of native design files is set per item.

### 7.7 Manufacturing records

- **Work instructions:** steps, images and required tools, tied to an assembly revision.
- **As-built records:** per serial number, the revision built, parts and lot numbers used, and step sign-offs.
- **Inspection and test records:** linked to procedures, requirements and serial numbers.

### 7.8 Import and export

- CSV import and export for parts and Bill of Design data.

## 8. Change control

### 8.1 Change orders

- A change order describes a reason for change, the affected items and the proposed changes.
- Opening a change order creates a working branch in the engineering repository. Edits to affected items are made on that branch.
- Draft items change freely. Released items and controlled pages change only through a change order.
- The owner starts a new draft revision of a released item when ready. Starting a revision does not require a change order; releasing it does.
- The change order record (state, approvals and signatures) is stored in PostgreSQL.

### 8.2 States

Draft, In Review, Approved, Released, Rejected, Cancelled.

### 8.3 Review and approval

- The review view shows every change the change order would make, compared with the current released data.
- Approvers are assigned per change order. Users whose role permits it can add or remove approvers.
- Each approval or rejection records the approver, time and comment.
- An approver confirms their decision by re-entering their password (electronic signature).
- The author cannot be the only approver.
- A change order cannot be approved while validation errors exist on the affected items.

### 8.4 Release

- A Release Authority releases an approved change order.
- Release merges the changes, makes each affected draft revision the released revision, marks the previous revision Superseded, and creates a signed baseline tag.
- Any baseline can be viewed exactly as released and compared with any other baseline.

### 8.5 Deviations and waivers

- A deviation or waiver approves a one-time departure from the released design for specified serial numbers or quantities, without changing the design.
- Uses the same approval process as change orders.

## 9. Work

### 9.1 Work item types

| Type | ID prefix | Purpose |
|---|---|---|
| Task | TSK | A unit of work with an owner and due date |
| Issue | ISS | A problem in the design, factory or a process |
| Nonconformance | NCR | A part or build that does not meet specification |
| Action | ACT | A follow-up from a review, meeting or audit |
| Epic | EPC | A group of related work items |

### 9.2 Fields

ID, title, description (markdown), type, status, priority, assignee, reporter, due date, labels, sprint, milestone, epic, estimate, time logged, watchers, attachments.

Links: blocks, blocked by, relates to, duplicates, affects (engineering item), found on (assembly or serial number), fixed by (change order).

### 9.3 Workflows

Default workflow for tasks, issues and actions:

Open, In Progress, In Review, Done, with Blocked and Cancelled available from any open state.

Nonconformance workflow:

| State | Description |
|---|---|
| Reported | Nonconformance recorded |
| Dispositioned | Decision recorded: use as is, rework, repair, scrap or return to vendor |
| Corrective Action | Root cause and corrective action in progress |
| Verified | Corrective action confirmed effective |
| Closed | Complete |

- A nonconformance whose fix changes the design requires a linked change order before it can close.
- Admins can edit workflows per type: states, allowed transitions, roles allowed to perform each transition, and required fields per state.

### 9.4 Views

| View | Description |
|---|---|
| Board | Columns by status with drag and drop |
| List | Sortable, filterable table with saved filters |
| My Work | Items assigned to the current user, overdue first |
| Timeline | Dated items over time, grouped by epic or milestone |
| Dashboard | Counts by status, aging, NCRs by part and supplier |

### 9.5 Planning

- **Sprints:** time-boxed periods with a goal, start and end date. Board filtered to the active sprint. Unfinished items move to the next sprint at close.
- **Milestones:** named target dates with progress based on linked items.
- **Dependencies:** blocked items show what they are waiting on.

### 9.6 Time tracking

- Users log time against work items with date, duration and note.
- Estimates compared with time logged per item, epic, sprint and user.
- Time reports exportable to CSV.

## 10. Notifications

- In-app inbox for assignments, mentions, status changes, approvals requested and due dates.
- Email notifications through a configured mail server, optional per user.
- Users watch any item or page to follow its changes.

## 11. Search

- Full-text search across pages, engineering items and work items.
- Filters by space, type, status, author and label.
- Backlinks on every record showing what links to it.

## 12. Time-series data

- DesignHub connects to InfluxDB for reading and writing time-series data.
- **Write:** test stations and equipment submit measurements through the DesignHub API, tagged with serial number, equipment, station and procedure.
- **Read:** charts on equipment, serial number and page views; production metrics such as throughput, yield and cycle time.
- A failed test result can open a nonconformance automatically.

## 13. Optional online services

Each service is disabled by default, configured by a Super Admin, and has a timeout so an unavailable service never blocks the application.

| Service | Purpose |
|---|---|
| Part data lookup | Retrieve manufacturer, datasheet, lifecycle and pricing data when a part is created; data is saved to the part record |
| Lifecycle and stock alerts | Scheduled check for parts becoming obsolete or low on stock |
| Remote repository sync | Push and pull the engineering repository to a remote git server for off-site backup |
| Email | Notification delivery |
| Update check | Reports when a new DesignHub version is available; never installs automatically |

## 14. Security and audit

- HTTPS for all access once a domain is configured. During local network testing, access uses the fixed IP address on the local network.
- Session cookies are secure, HTTP-only and expire after inactivity. "Keep me signed in" keeps a session for 1 day.
- Protection against cross-site request forgery on every form and API call.
- Rate limiting on sign-in.
- Audit log of sign-ins, permission changes, approvals, releases, settings changes and data exports.
- Commits in the engineering repository are attributed to the signed-in user.

## 15. Backup and recovery

- Scheduled backups of PostgreSQL, the engineering repository and InfluxDB to the `backups` volume.
- Proxmox virtual machine backups as a second layer.
- Documented restore procedure, tested before production use.
- The search index and link graph are rebuilt automatically after a restore.

## 16. Release plan

### 16.1 Version 1

| Area | Included |
|---|---|
| Deployment | Docker Compose on Proxmox, local network access |
| Sign-in | Local accounts, two-factor authentication |
| Roles | Super Admin, Admin, Release Authority, Approver, Editor, Contributor, Viewer |
| Spaces and Pages | Spaces, page tree, all four editing modes, Bill of Design and work list blocks, comments, history, controlled pages |
| Engineering data | Requirements, parts, assemblies, procedures, documents, Bill of Design, where-used, traceability, validation, attachments, manufacturer list |
| Change control | Change orders with approvals, electronic signature, release and baselines |
| Work | Tasks, issues, NCRs, actions, epics; board, list and My Work views; default and NCR workflows |
| Notifications | In-app inbox and watchers |
| Search | Full-text search and backlinks |
| Backup | Scheduled backups |

### 16.2 Version 2

- Google sign-in (once the domain is in place)
- Vendor and Contractor roles with company scoping and expiry
- Custom roles and editable workflows
- Sprints, milestones, timeline view and time tracking
- Templates, inline comments, PDF export
- Email notifications
- CSV import and export

### 16.3 Version 3

- InfluxDB integration: charts, test data, production metrics
- Work instructions, as-built records, inspection and test records
- Supplier, equipment and location item types
- Effectivity
- Deviations and waivers
- Optional online services

## 17. Open items

| Item | Status |
|---|---|
| Domain name and HTTPS certificate | To be obtained. Required before Google sign-in. |
| Mail server for email notifications | To be decided before version 2 |
| Detailed data model (fields and relations per type) | Defined in [DATA-MODEL.md](DATA-MODEL.md) |
| Screen layouts | Defined in [SCREENS.md](SCREENS.md) |
| Test plan for version 1 | Defined in [TEST-PLAN.md](TEST-PLAN.md) |
