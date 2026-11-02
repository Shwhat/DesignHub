# Records and audit

## History

- Every save of an engineering item or page is recorded with user, time and comment.
- Any two versions can be compared line by line.
- Any earlier version of a page can be restored. A restore is recorded as a new version.
- Saves use conflict detection. If a record changed after you opened it, your save is refused and the differences are shown, so one person's save never overwrites another's.

## Deletion

- Released records are never deleted. They are made Obsolete or Cancelled.
- A draft that has never been released can be deleted. Its number is retired.
- Users and spaces are never deleted. They are disabled or archived, so history stays attributed.
- Comments are never deleted. The author or an Admin can hide a comment, and the hide is logged.
- Values in Admin lists are deactivated, never deleted.

## Audit log

The audit log is an append-only record of:

- sign-ins, failed sign-ins and sign-outs
- user, role and permission changes
- approvals, rejections, releases and failed signature attempts
- settings changes (secrets are never written to the log)
- creating, deleting and locking records, and changes to lists, spaces and members
- backups and search index rebuilds
- every CSV export, CSV import and PDF export
- company, supplier link, workflow, template, sprint and milestone changes

Admins with audit permission can search the log in Admin, Audit log.

## Where data is stored

| Data | Storage |
|---|---|
| Parts, assemblies, Bills of Design, requirements, procedures, documents, pages | Markdown files in a git repository |
| Edits made under a change order | The change order's own branch in that repository |
| Released baselines | Signed tags in that repository |
| Attachment files | A file store on the server, referenced from the repository by checksum |
| Change order records, approvals and signatures | PostgreSQL |
| Work items, comments, notifications, watchers | PostgreSQL |
| Companies, supplier links, workflows, sprints, milestones, time entries, templates, CSV import records | PostgreSQL |
| Users, roles, spaces, settings, lists, audit log | PostgreSQL |
| Search index and link graph | PostgreSQL, rebuilt from the repository on startup and after every save |

Engineering data is plain Markdown in standard git, so it remains readable without DesignHub.

## Export and import

- Every table has Export CSV, which saves the rows shown with the current filters and columns. Each record type also has a full export with every field. Exports contain only records you can see.
- CSV import is available for parts, Bill of Design lines, requirements, work items, manufacturers and companies. Column names match the export, so an exported file can be changed and imported again.
- Every import shows a preview first. Nothing is saved while any row has an error. Imported engineering items are created as Drafts, and released items are never changed by an import.
- Each import is kept with its file in Admin, Imports.
- PDF exports show the record as it is on screen, with the record ID, revision and status in the header, and "Not released" on every page of a controlled page or item that is not released. Change order PDFs list each approval with its time and meaning.

## Backups

- A backup is taken daily at the scheduled hour. The number of copies kept is set in Admin, System (default 14).
- Each backup contains the database, the engineering repository and the attachment files.
- An Admin can take a backup at any time in Admin, System.
- Restore is done on the server and rebuilds the search index automatically.
