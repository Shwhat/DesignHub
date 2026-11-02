# Overview

DesignHub is a self-hosted engineering system. It brings three tools into one connected application:

| Area | What it holds |
|---|---|
| **Pages** | Documentation organized in spaces, with a page tree, comments and full history |
| **Work** | Tasks, issues, nonconformances (NCRs), actions and epics, with boards and lists |
| **Library and Bill of Design** | Controlled engineering data: parts, assemblies, requirements, procedures and documents, with revisions, change orders and released baselines |

Every record can link to every other record. A requirement links to the design that satisfies it, the design links to its parts, an NCR links to the part it was found on, and a page can show any of this live.

## Principles

1. **No dependence on the internet.** DesignHub runs entirely on the local network. Online features, such as Google sign-in, are optional and never required for the application to work.
2. **Portable engineering data.** Controlled engineering data is stored as plain Markdown files in a standard git repository, readable without DesignHub.
3. **Everything is traceable.** Every change records who made it, when and why.
4. **Own branding only.** The application shows only DesignHub branding. The one exception is the Google sign-in button, whose wording and logo Google requires.
5. **Bundled assets.** Fonts, scripts and styles ship with the application. Nothing is loaded from outside servers.

## In this guide

| Page | Covers |
|---|---|
| [Capabilities](/about/capabilities) | What DesignHub can do today, and what is planned |
| [Numbering](/about/numbering) | How every record is identified |
| [Revisions and release](/about/revisions) | Revisions, statuses, attachments and Bill of Design rules |
| [Change control](/about/change-control) | Change orders, approvals, electronic signatures and baselines |
| [Work and NCRs](/about/work) | Work item and nonconformance workflows |
| [Access and sign-in](/about/access) | Roles, permissions, record locking, sign-in and sessions |
| [Records and audit](/about/records) | History, deletion, audit log, storage and backups |
| [Record fields](/about/fields) | The fields of each record type |
| [Technical](/about/technical) | Architecture and third-party components |
| [Release notes](/about/release-notes) | What changed in each update |
