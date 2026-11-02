# Release notes

The installed version is shown at the top of this guide.

## 2.0.0 (2026-11-02): Version 2

- Companies: vendors, contractors and customers with supplier status, contacts, supplied items and NCRs. Suppliers can be set on released items without a change order.
- Vendor accounts see only released revisions of items their company supplies, the NCRs naming their company and the actions assigned to them. They respond to supplier requests and add supplier documents.
- Contractor accounts work only in the spaces they belong to.
- Account expiry, with notices to Admins 14 days and 1 day before.
- Internal comments, hidden from Vendors.
- Custom roles copied from existing roles.
- Editable workflows for tasks, issues, actions, epics and NCRs, with categories, transitions by role and required fields. The NCR disposition and change order rules stay in force.
- Sprints with a backlog, milestones with health, a timeline, estimates, time logging, My time and time reports.
- Page templates, with six that come with DesignHub.
- Inline comments on selected page text.
- PDF export of pages and page trees, engineering items, change orders, work items and NCRs.
- CSV export from every table and full exports of every record type; CSV import with a preview for parts, BOM lines, requirements, work items, manufacturers and companies.
- New PDF service container in the Docker Compose file. Pull the new compose file before updating.

## 1.2.1 (2026-11-01): New look

- DesignHub now shares OpenSchem's look: the same colors, light and dark themes, type sizes, spacing, buttons, menus, dialogs and lists.
- The top bar is dark, matching OpenSchem's menu bar.
- Text symbols used as icons are replaced by drawn icons, including the sidebar, page tree, work item types, sorting, locks and close buttons.
- Desktop screens are more compact. Phones and tablets keep larger touch targets.
- The interface uses the device's own font instead of a bundled typeface.

## 1.2.0 (2026-11-01): About guide

- New About guide, reachable from the sidebar and the profile menu, describing what DesignHub does, the rules it follows and its record specifications.

## 1.1.0 (2026-10-31): Google sign-in

- Google sign-in, limited to allowed Google Workspace domains and individually allowed addresses, configured in Admin, System.
- New Google users start as Viewer with no space access.
- Existing accounts link to Google by email on first Google sign-in.
- Password sign-in can be turned off for everyone.
- Signing PIN for Google users, used for approvals, releases and NCR decisions. Admins can clear a forgotten PIN.
- Recovery command on the server for a locked-out Super Admin.

## 1.0.1 (2026-10-31): Fixes

- Library now shows its columns and rows when no column choice has been saved. The work List view had the same fault.
- Phone search icon resized to match the other top bar icons.
- Banner text, such as on released change orders, no longer shows gaps around links.

## 1.0.0 (2026-10-30): Version 1

First release, with:

- Library of parts, assemblies, requirements, procedures and documents, with numbering, revisions, attachments and the manufacturer list.
- Bill of Design with extended quantities, where-used and validation.
- Traceability and backlinks.
- Change orders with approvals, electronic signatures, release and signed baselines.
- Spaces and pages with four editing modes, live blocks, comments, history and controlled pages.
- Tasks, issues, NCRs, actions and epics with board, list and My Work views.
- Notifications, watchers, live updates and full-text search.
- Users, roles, record locking, audit log, scheduled backups and restore.
