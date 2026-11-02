# Capabilities

This page lists what DesignHub does in the current version, followed by what is planned.

## Available now

### Library and engineering items

- Parts (production, reference and purchased), assemblies, requirements, procedures and documents.
- Automatic numbering that follows the [numbering rules](/about/numbering).
- Draft editing with autosaved drafts, conflict detection on save and full version history.
- Attachments on each revision, with file roles and release warnings for missing expected files. PDFs and images preview in the browser.
- Manufacturer list for purchased part numbers.
- Conversion of a reference part to a production part.
- Library list with saved filters and a choice of columns per tab.
- Companies (vendors, contractors, customers) with supplier status, contacts and the items each one supplies. Suppliers can be changed on released items without a change order.

### Bill of Design

- Multi-level Bill of Design with find numbers, quantities, reference designators and line notes.
- Extended quantities and a flattened parts list.
- Where-used for every part and assembly.
- Validation: missing items, cycles, released assemblies that contain unreleased items, and reference designator counts.

### Traceability

- Typed links between records, with the reverse link shown automatically on the target.
- Traceability view showing each requirement's satisfying design and verifying procedures, with gaps.
- Backlinks on every record.

### Change control

- Change orders with affected items, approvers, validation and a side-by-side review of every change.
- Electronic signatures on approvals, releases and NCR dispositions.
- Release creates the new released revisions and a signed baseline.
- Baselines viewable exactly as released and comparable with each other.
- Controlled pages that change only through a change order.

### Spaces and pages

- Spaces with their own members, roles, home page and page tree.
- Four editing modes: Visual, Split, Source and Preview toggle.
- Live blocks inside pages: Bill of Design, work list, item card, traceability and change orders.
- Links to any record by writing its ID in double brackets, for example `[[100042-001]]`.
- Page comments with replies, @mentions and resolve.
- Version history with comparison between any two versions, and restore.
- Drag and drop to reorder and move pages.
- Page templates with placeholders, company-wide or per space. Six templates come with DesignHub.
- Inline comments on selected text, highlighted in the page, and internal comments hidden from Vendors.

### Work

- Tasks, issues, actions, epics and nonconformances (NCRs).
- Board and list views per space, with separate NCR board and list.
- My Work: items assigned to you, overdue first.
- NCR workflow with signed disposition approvals and a required change order when a design change is needed.
- Creating a change order directly from an NCR.
- Editable workflows for tasks, issues, actions, epics and NCRs: states, categories, transitions by role and required fields.
- Sprints with a backlog, start and close; milestones with progress and health; a timeline by epic or milestone.
- Estimates, time logging, My time and time reports.
- Supplier responses on NCRs, and supplier documents.

### Notifications and search

- In-app notification inbox and watchers on any record.
- Live updates: changes made by others appear without reloading.
- Full-text search across pages, engineering items and work items, with filters.

### Export and import

- CSV export from every list and table, and a full export of every record type.
- CSV import of parts, Bill of Design lines, requirements, work items, manufacturers and companies, with a preview. Nothing is saved while any row has an error.
- PDF export of pages (with the pages below them), engineering items, change orders, work items and NCRs.

### Administration

- Users, roles and space membership.
- Custom roles copied from existing roles.
- Vendor and Contractor accounts linked to a company, with account expiry.
- Admin lists (units of measure, document types, page types and others) and file roles.
- Sign-in message, allowed addresses and change order rejection behavior.
- Google sign-in limited to allowed domains and addresses, with optional removal of password sign-in.
- Audit log viewer.
- Scheduled backups, on-demand backup and system status.

### Personal settings

- Display name, email, time zone and theme (light, dark or follow device).
- Default editing mode, Home layout and notification choices.
- Active sessions with sign-out of other devices.

## Planned

These are part of the release plan and are not available yet.

| Version | Planned features |
|---|---|
| 2 | Email notifications, once a mail server is chosen |
| Later | Work item dashboard |
| 3 | Time-series data with charts, test data and production metrics; work instructions, as-built records, inspection and test records; supplier, equipment and location records; effectivity; deviations and waivers; optional online part data and lifecycle services |
