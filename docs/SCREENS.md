# DesignHub Screen Layouts

| | |
|---|---|
| Document | DesignHub version 1 and version 2 screen layouts |
| Version | 0.3 (draft for review) |
| Date | 2026-11-01 |
| Owner | Devon |
| Status | Draft |
| Related | [SPEC.md](SPEC.md), [DATA-MODEL.md](DATA-MODEL.md) |

## Contents

1. General layout
2. Sign-in
3. Home and My Work
4. Library browser
5. Engineering item view
6. Assembly view
7. Item create and edit
8. Change order view, review and approval
9. Space home and page tree
10. Page view and editing modes
11. Work board and list views
12. Work item and NCR view
13. Search
14. Notifications inbox
15. Admin
16. User profile and settings
17. About guide
18. Companies (v2)
19. Vendor and Contractor views (v2)
20. Sprints, milestones and timeline (v2)
21. Time tracking (v2)
22. Templates (v2)
23. Inline comments (v2)
24. CSV import and export (v2)
25. PDF export (v2)
26. Workflow editor and custom roles (v2)
27. Email notifications (v2)
28. Open items

---

## 1. General layout

| Item | Decision |
|---|---|
| Navigation | Top bar and left sidebar on every screen after sign-in |
| Top bar | Search, notifications bell, profile menu |
| Left sidebar | Home, My Work, Library, Change orders, Search, Notifications, About, space switcher, page tree and Work entries of the current space |
| Devices | Desktop, tablet and phone, with responsive layouts for all three in version 1 |
| Theme | Light and dark, chosen in user settings, with a follow-device option |
| Density | Comfortable |
| Branding | DesignHub only |

### 1.1 Common record layout

Engineering items, change orders, work items and NCRs share one layout:

| Area | Content |
|---|---|
| Header | ID, revision where applicable, status or state, title, Actions menu |
| Tabs | Specific to the record type |
| Comments | Side panel on desktop; opened from a Comments button on tablet and phone |
| Editing | In place on the record view. Save and Cancel appear in the header. |
| Drafts | Edits are autosaved as a private draft. Save records the version with a comment. |

## 2. Sign-in

| Area | Content |
|---|---|
| Top | DesignHub logo |
| Admin message | Text set by an Admin, shown above the form. Hidden when empty. |
| Google sign-in | When configured: "Sign in with Google" button and "Keep me signed in" checkbox above the password form. The password form opens from a "Sign in with a password" link, or "Recovery sign-in" when password sign-in is turned off. |
| Form | Username, Password, "Keep me signed in" checkbox, Sign in button |
| Forgot password | Link below the form. Shows "Contact your administrator." |
| Errors | One message for any wrong username or password: "Username or password is incorrect." A rate-limit message after repeated failures. Google refusals return to this screen with a message: address not allowed, account disabled, address shared by several accounts, request expired or cancelled. |
| Keep me signed in | Keeps the session for 1 day. Fixed, not configurable. |
| Theme | Follows the device setting |
| Phone | Single column, full width |

## 3. Home and My Work

### 3.1 Home

| Area | Content |
|---|---|
| Sections | Assigned work items, change orders awaiting my approval, my engineering items in Draft or In Review, recently viewed records, recent activity in my spaces, unread notifications |
| Customizing | Each section can be turned on or off and reordered in user settings |
| Reset | A major update resets the Home layout to default. Other user settings are kept. |
| Tablet | Sections in two columns |
| Phone | Sections stacked in one column |

### 3.2 My Work

| Area | Content |
|---|---|
| Content | Work items assigned to the current user |
| Sort | Overdue first |
| Grouping | User chooses: type, space or none |
| Group order | User can change the order of groups |

## 4. Library browser

| Area | Content |
|---|---|
| Tabs | Parts, Assemblies, Requirements, Procedures, Documents, Companies (v2) |
| Part columns | ID, revision, name, status, owner, home space, part kind |
| Default filter | Current revision only. Toggle to include Superseded and Obsolete. |
| Saved filters | Yes, per user |
| Columns | User can choose and reorder columns |
| Phone | Full table with horizontal scroll. Toggle for a simple list (ID, name, status). |
| Export (v2) | Export CSV button above every tab's table (section 24) |
| Import (v2) | Import CSV in the tab's Actions menu for Parts, Requirements and Companies (section 24) |

## 5. Engineering item view

Applies to parts, requirements, procedures and documents. Uses the common record layout (section 1.1).

| Area | Content |
|---|---|
| Header | ID, revision, status, name, count badge of linked work items, Actions menu |
| Actions menu | Start new revision, Edit, Add to change order, Watch, Lock |
| Tabs | Details, Files, Links, Revisions, History |
| Linked work items | Count badge in the header and listed in the Links tab |
| Older revision | Banner: "You are viewing rev 01 (Superseded). Current: rev 03" with a link to the current revision |

## 6. Assembly view

The engineering item view (section 5) with three more tabs.

| Area | Content |
|---|---|
| Extra tabs | BOM, Where-used, Validation |
| BOM tab | Tree / Flat toggle. Opens on Tree. |
| Tree | Structure level by level. A part used in several sub-assemblies appears under each one. |
| Flat | Every unique part once with its total quantity across all levels |
| BOM columns | Find number, ID, revision, name, status, quantity, unit, reference designators, extended quantity |
| Tree expansion | Remembers the user's last choice |
| Validation | Listed in the Validation tab and shown as an icon on each affected BOM line |
| Editing | BOM lines are edited through the item edit mode only |

## 7. Item create and edit

### 7.1 Create

| Step | Content |
|---|---|
| 1 | Pick type: Part, Assembly, Requirement, Procedure, Document |
| 2 | For parts and assemblies, pick kind: Production, Reference, COTS |
| 3 | Fill in the form for that type |
| 4 | Click Create. The number is assigned at this point. |

### 7.2 Production number

| Item | Decision |
|---|---|
| Display | The form shows the next available number, for example 100043-001 |
| Changing | The user can change both the 6-digit base and the 3-digit suffix |
| Rule | The full number (base and suffix) must not already be used or retired |
| Counter | The counter skips any base number already used in any form |
| Assignment | The number is reserved only when the user clicks Create |

### 7.3 Edit

| Item | Decision |
|---|---|
| Mode | In place on the item view. Field values on each tab become inputs. Save and Cancel in the header. |
| Saving | Autosaved private draft. Save records the version with a comment. |
| BOM lines | Add by ID with search as you type. Drag to reorder find numbers. |

## 8. Change order view, review and approval

Uses the common record layout (section 1.1).

| Area | Content |
|---|---|
| Header | ID, state, title, Actions menu |
| Tabs | Details, Affected items, Changes, Approvals, Validation, History |
| Changes tab | Toggle between side-by-side (released left, proposed right) and single-column with additions and removals highlighted. Single-column on phone. |
| Decision bar | Approve and Reject in a fixed bar at the bottom of the screen, visible while scrolling. Shown only to assigned approvers. |
| Electronic signature | Pop-up asking for a comment and the password (local accounts) or signing PIN (Google accounts). A Google user without a PIN is sent to Settings, Security. |
| Approvals tab | Each approver with decision, time and comment. "Add approver" button for permitted roles. |

## 9. Space home and page tree

| Area | Content |
|---|---|
| Space switcher | Top of the left sidebar |
| Page tree | In the left sidebar below the space switcher. Expands on entering the space. |
| Space home | Description, pinned links, recent changes, open work item counts, open change orders |
| Pinning | Space Editors and above |
| Page tree actions | New child page, move, rename. From a right-click menu, a "..." menu on each page, and drag to move. |
| New page dialog (v2) | Title, parent page, and a template picker listing the space's and company-wide active templates with their descriptions. "Blank page" is first. |
| Space home (v2) | Adds the active sprint (goal, days left, progress) and upcoming milestones with health |

## 10. Page view and editing modes

| Area | Content |
|---|---|
| Layout | Breadcrumbs, title, body. Comment side panel. |
| Page details | Collapsible info panel: ID, type, owner, controlled, revision |
| Mode switcher | Dropdown in the editor toolbar: Visual, Split, Source, Preview toggle |
| Editing | In place. Autosaved private draft. Save records a version with a comment. |
| Controlled pages | Edit becomes "Edit under change order". The user picks an existing change order or creates one. |
| Live blocks | Inserted from an "Insert block" toolbar menu or by typing "/" in the body |
| Phone | Editing allowed in all four modes. Split shows markdown above the preview. |
| Inline comments (v2) | Section 23 |
| Export PDF (v2) | Actions menu: "Export PDF..." with a choice of this page only, or this page and the pages below it (section 25) |

## 11. Work board and list views

| Area | Content |
|---|---|
| Sidebar entries | Board, List, NCR Board, NCR List under each space. Each page keeps its own filters. v2 adds Timeline, Sprints and Milestones. |
| Board cards | ID, title, type icon, priority, assignee, due date |
| Board | Columns by status for tasks, issues, actions and epics |
| NCR Board | Columns by NCR workflow state |
| Swimlanes | Off by default. Can be added by assignee, epic or priority. |
| List | Saved filters and user-chosen columns |
| Phone | Board pages open as the list view |
| Board columns (v2) | Follow the workflow's states in order (section 26) |
| Sprint filter (v2) | Board and List have a sprint picker: Active sprint (default when one exists), a named sprint, No sprint, All |
| List columns (v2) | Adds Estimate, Time logged, Remaining, Sprint, Milestone |
| Export (v2) | Export CSV on List and NCR List |

## 12. Work item and NCR view

Uses the common record layout (section 1.1).

| Area | Content |
|---|---|
| Header | ID, status dropdown, title, Actions menu |
| Status dropdown | Lists only the allowed next states |
| Work item tabs | Details, Links, Attachments, History |
| NCR tabs | Report, Disposition, Corrective action, Verification, Links, Attachments, History |
| NCR stage fields | Fields of later stages are visible but empty until reached |
| Disposition signature | Same pop-up as change order approval |
| Create change order | Action on an NCR. Creates a change order prefilled with the found-on item and links it to the NCR. |
| Planning fields (v2) | Estimate, Sprint and Milestone in Details. Time logged and Remaining shown beside Estimate. |
| Time tab (v2) | Section 21 |
| NCR Supplier (v2) | Supplier company picker on the Report tab. A Supplier response tab shows the request and due dates and the supplier's root cause and corrective action. "Request supplier response..." in the Actions menu sets the dates and notifies the supplier's Vendors. |
| Status dropdown (v2) | Lists the transitions the workflow allows the user. Missing required fields open a dialog asking for them. |
| Export PDF (v2) | Actions menu |

## 13. Search

| Area | Content |
|---|---|
| Top bar | Quick results grouped by type as the user types. Enter opens the Search page. |
| Exact ID | Typing an exact ID opens that record |
| Search page | Filters in a left panel: space, type, status, author, label. Results in the main area. |
| Phone | Filters behind a Filters button |
| Result | Type icon, ID, title, status, space, snippet with the match highlighted |
| Saved searches | Yes, per user |

## 14. Notifications inbox

| Area | Content |
|---|---|
| Top bar | Bell with unread count. Opens a drop-down of recent notifications with a "View all" link. |
| Inbox page | Filters: All, Unread, Mentions, Approvals |
| Notification | Event icon, summary, record ID and title, triggered by, time |
| Actions | Mark read, mark unread, mark all read, dismiss |
| Click | Opens the record and marks the notification read |

## 15. Admin

| Area | Content |
|---|---|
| Entry | Admin item in the profile menu, shown to Admins and Super Admins |
| Admin menu | Users, Roles, Workflows (v2), Templates (v2), Spaces, Manufacturers, Lists, File roles, Imports (v2), Sign-in message, Audit log, System |
| Users | Table: username, display name, email, library role, active, last sign-in. v2 adds company and account expiry columns, with expired and expiring-soon accounts highlighted. Detail page per user for space roles, password reset (local accounts), signing PIN clear (Google accounts) and disable. Table includes the sign-in method. |
| Roles | Permission grid. Roles as columns, permissions as rows. v2: editable for custom roles (section 26). |
| Lists | One page with a list picker. Each list is an editable table with drag to reorder and an Active toggle. |
| Super Admin sections | System settings, backups and database status. In the same menu, visible only to Super Admins. |
| System | Google sign-in (client ID, client secret, public address, redirect URI to register with Google, allowed domains, allowed individual addresses, show Google sign-in, turn off password sign-in), allowed addresses, change order rejection behavior, backups and database status. v2: mail server settings (section 27). |

## 16. User profile and settings

| Area | Content |
|---|---|
| Profile menu | My profile, Settings, Admin (if permitted), About DesignHub, Sign out |
| Profile | Display name, email, time zone |
| Security | Local accounts: change password. Google accounts: signing PIN set or change. Both: active sessions with sign out of other devices. |
| Appearance | Light, dark, follow device |
| Editing | Default editing mode |
| Home layout | Sections on or off and their order |
| Notifications | Which events notify the user. v2: email delivery Off, Immediately or Daily summary, and which events are emailed (section 27). |
| Time (v2) | Link to My time (section 21) |

## 17. About guide

| Area | Content |
|---|---|
| Entry | About in the left sidebar and About DesignHub in the profile menu. Signed-in users only. |
| Header | "About DesignHub" and the installed version |
| Page list | Left column listing every guide page. On phones, a page picker at the top. |
| Pages | Overview, Capabilities, Numbering, Revisions and release, Change control, Work and NCRs, Access and sign-in, Records and audit, Record fields, Technical, Release notes |
| Page body | Rendered Markdown with links between guide pages, and Previous and Next links at the bottom |
| Source | Markdown files in `web/src/about`, bundled with each build. File `NN-slug.md` is page `/about/slug`, ordered by NN, titled by its first heading. |
| Unknown page | Not found screen |

## 18. Companies (v2)

| Area | Content |
|---|---|
| List | Library, Companies tab. Columns: ID, name, kind, supplier status, active, number of supplied items |
| Company view | Common record layout. Tabs: Details, Supplied items, NCRs, People, Attachments, History |
| Supplied items | Parts and assemblies with a `supplied by` link, with revision and status. "Add items..." opens a record picker. |
| People | Users linked to the company, with role and account expiry. Admins can add a Vendor or Contractor user from here. |
| Supplier status warning | Parts and NCRs linked to a company that is not Approved show an amber banner with the status |
| Part view | Suppliers row in Properties, edited in place without a change order |

## 19. Vendor and Contractor views (v2)

| Area | Vendor | Contractor |
|---|---|---|
| Sidebar | Home, Supplied items, NCRs, Notifications, About | Home, My Work, Library, Search, Notifications, About, and their spaces |
| Home | Open NCRs needing a response, actions assigned, recently changed supplied items | Same as other users, limited to their spaces |
| Supplied items | Library table limited to their company's items, released revisions only | Library limited to items whose home space is one of their spaces |
| NCR view | Report tab read only, Supplier response tab editable, Attachments tab for supplier documents, Comments without Internal comments | As other users in their spaces |
| Top bar | Company name beside the avatar | Company name beside the avatar |
| Expiry | A banner from 14 days before expiry: "Your access ends on {date}." | Same |
| Search | Only their records | Only their records |

## 20. Sprints, milestones and timeline (v2)

**Sprints page** (sidebar, Work, Sprints)

| Area | Content |
|---|---|
| List | Active sprint first, then Planned in date order, then Closed. Columns: name, dates, goal, items, done, estimate, logged. |
| Backlog | Below the list: open items with no sprint. Drag items onto a sprint, or use "Add to sprint" on selected rows. |
| Start sprint | Button on a Planned sprint. Confirms dates and records the committed items. |
| Close sprint | Button on the Active sprint. Dialog: unfinished items go to the next Planned sprint or to no sprint. |
| Sprint view | Goal, dates, committed and completed counts, estimate against logged, item list grouped by state |

**Milestones page** (sidebar, Work, Milestones)

| Area | Content |
|---|---|
| List | Name, target date, owner, health, progress bar, items |
| Milestone view | Common record layout. Progress by count and by estimate, linked items, history |
| Health colors | On track green, At risk amber, Late red, Done grey |

**Timeline** (sidebar, Work, Timeline)

| Area | Content |
|---|---|
| Rows | Grouped by epic (default) or by milestone. Items without a group at the end. |
| Bars | Epics from start to target date; work items from created or start to due date; milestones as diamonds; sprints as shaded bands across the top |
| Scale | Week, Month, Quarter. Today marked with a vertical line. |
| Editing | Drag a bar end to change its date, for users who can edit the item. Undo with Ctrl+Z. |
| Dependencies | `blocks` links drawn as arrows between bars |
| Phone | List grouped by month instead of bars |

## 21. Time tracking (v2)

| Area | Content |
|---|---|
| Work item Time tab | Estimate, logged, remaining. Table of entries: date, person, hours, note. "Log time" form at the top: date (today), hours, note. |
| Quick log | "Log time..." in the work item Actions menu and on board cards (right-click) |
| My time | Settings, Time, or the profile menu: the user's entries for a week, with totals per day and per item, and a form to add entries |
| Time report | Work, Time report in each space: filters for date range, people, epic, sprint, milestone, type. Group by person, item, epic or sprint. Columns: estimate, logged, difference. Export CSV. |
| Rules | Section 7.24 of DATA-MODEL.md |

## 22. Templates (v2)

| Area | Content |
|---|---|
| Admin, Templates | Table: name, scope, page type, active, changed on. New template, copy, deactivate. |
| Space templates | Space home, Actions, Templates: the same table for that space, for space Admins |
| Template editor | Same editor and modes as pages. Fields: name, scope, description, page type, title pattern. A Placeholders menu inserts {date}, {user}, {space}, {title}. |
| Use | New page dialog (section 9) |

## 23. Inline comments (v2)

| Area | Content |
|---|---|
| Start | Select text in a page or a long text field in view mode. A "Comment" button appears beside the selection. |
| Highlight | Commented text has a light amber underline. Clicking it opens the comment in the side panel. |
| Panel | Inline comments listed in document order above page comments, each showing its quoted text |
| Text changed | When the quoted text is no longer in the current version, the comment shows "Text changed" and keeps its quote |
| Resolve | Resolving removes the highlight; the comment stays in the panel under Resolved |
| Phone | Long-press selection, then "Comment" in the selection bar |

## 24. CSV import and export (v2)

| Area | Content |
|---|---|
| Export CSV | Button on every table: Library tabs, BOM, where-used, traceability, change orders, work List and NCR List, sprints, milestones, time report, search results, Admin users and audit log. Exports the rows shown with current filters, sort and columns. |
| Full export | Library and work lists, Actions menu, "Export all fields (CSV)..." |
| Import entry | Actions menu of the Parts, Requirements and Companies tabs; Bill of Design tab of a draft assembly; work List; Admin, Manufacturers. "Import CSV..." |
| Step 1, file | Choose a file. "Download a blank file" gives the header row for this record type. |
| Step 2, mode | Create only, or Create and update. BOM: Add to BOM, or Replace BOM. |
| Step 3, preview | Table of rows with an outcome column: Create, Update, Error. Errors listed with row number and reason. Import is disabled while there are errors. "Download errors" gives the rows with an error column. |
| Step 4, result | Counts created and updated, link to the import record |
| Admin, Imports | Table of past imports: date, user, type, rows, created, updated, file |

## 25. PDF export (v2)

| Area | Content |
|---|---|
| Entry | "Export PDF..." in the Actions menu of pages, engineering items, change orders, work items and NCRs |
| Options | Pages: this page only, or with pages below it. Items: include Bill of Design, include files list. Paper: Letter or A4 (user default). |
| Progress | The dialog shows progress and then downloads the file. Exports of more than 50 pages run in the background and notify the user when ready. |
| Layout | Section 7.28 of DATA-MODEL.md |
| Print view | "Print..." in the same menu opens the browser print dialog with the same layout |

## 26. Workflow editor and custom roles (v2)

**Admin, Workflows**

| Area | Content |
|---|---|
| Type picker | Task, Issue, Action, Epic, NCR |
| States | Ordered table: name, category, color, required fields. Drag to reorder. Add state. The NCR Disposition state shows a lock icon and cannot be removed. |
| Transitions | Grid with From states as rows and To states as columns. Each cell is off or on; when on, a role list and the "Assignee may always move" toggle. |
| Preview | Diagram of states and transitions, drawn from the table |
| Save | Shows items affected. If a removed state holds items, a mapping step asks for a new state for each. Saving raises the workflow version. |
| History | Earlier versions with who changed them and when |

**Admin, Roles**

| Area | Content |
|---|---|
| Grid | As in v1, with custom roles as extra columns |
| Copy role | "Copy..." on a role column: name, applies to. Super Admin cannot be copied. |
| Edit | Custom role columns have checkboxes. Built-in role columns stay read only. |
| Deactivate | Custom roles in use are deactivated; unused ones can be deleted |

## 27. Email notifications (v2)

| Area | Content |
|---|---|
| Settings, Notifications | Email delivery: Off, Immediately, Daily summary at a chosen hour. Per event: in-app and email checkboxes. |
| Admin, System, Mail | Mail server settings (to be decided), sender address, "Send test email" |
| Email content | Subject: event and record ID. Body: summary, record title, link. No other record content. |

## 28. Open items

| Item | Status |
|---|---|
| Meaning of "major update" for the Home layout reset | To be defined |
