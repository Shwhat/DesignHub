# DesignHub Screen Layouts

| | |
|---|---|
| Document | DesignHub version 1 screen layouts |
| Version | 0.2 (draft for review) |
| Date | 2026-10-30 |
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
17. Open items

---

## 1. General layout

| Item | Decision |
|---|---|
| Navigation | Top bar and left sidebar on every screen after sign-in |
| Top bar | Search, notifications bell, profile menu |
| Left sidebar | Home, My Work, Library, Search, space switcher, page tree and Work entries of the current space |
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
| Form | Username, Password, "Keep me signed in" checkbox, Sign in button |
| Forgot password | Link below the form. Shows "Contact your administrator." |
| Errors | One message for any wrong username or password: "Username or password is incorrect." A rate-limit message after repeated failures. |
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
| Tabs | Parts, Assemblies, Requirements, Procedures, Documents |
| Part columns | ID, revision, name, status, owner, home space, part kind |
| Default filter | Current revision only. Toggle to include Superseded and Obsolete. |
| Saved filters | Yes, per user |
| Columns | User can choose and reorder columns |
| Phone | Full table with horizontal scroll. Toggle for a simple list (ID, name, status). |

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
| Electronic signature | Pop-up asking for password and comment |
| Approvals tab | Each approver with decision, time and comment. "Add approver" button for permitted roles. |

## 9. Space home and page tree

| Area | Content |
|---|---|
| Space switcher | Top of the left sidebar |
| Page tree | In the left sidebar below the space switcher. Expands on entering the space. |
| Space home | Description, pinned links, recent changes, open work item counts, open change orders |
| Pinning | Space Editors and above |
| Page tree actions | New child page, move, rename. From a right-click menu, a "..." menu on each page, and drag to move. |

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

## 11. Work board and list views

| Area | Content |
|---|---|
| Sidebar entries | Board, List, NCR Board, NCR List under each space. Each page keeps its own filters. |
| Board cards | ID, title, type icon, priority, assignee, due date |
| Board | Columns by status for tasks, issues, actions and epics |
| NCR Board | Columns by NCR workflow state |
| Swimlanes | Off by default. Can be added by assignee, epic or priority. |
| List | Saved filters and user-chosen columns |
| Phone | Board pages open as the list view |

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
| Admin menu | Users, Roles, Spaces, Manufacturers, Lists, File roles, Sign-in message, Audit log, System |
| Users | Table: username, display name, email, library role, active, last sign-in. Detail page per user for space roles, password reset and disable. |
| Roles | Read-only permission grid. Roles as columns, permissions as rows. |
| Lists | One page with a list picker. Each list is an editable table with drag to reorder and an Active toggle. |
| Super Admin sections | System settings, backups and database status. In the same menu, visible only to Super Admins. |
| System | Allowed addresses, change order rejection behavior, backups and database status |

## 16. User profile and settings

| Area | Content |
|---|---|
| Profile menu | My profile, Settings, Admin (if permitted), Sign out |
| Profile | Display name, email, time zone |
| Security | Change password, active sessions with sign out of other devices |
| Appearance | Light, dark, follow device |
| Editing | Default editing mode |
| Home layout | Sections on or off and their order |
| Notifications | Which events notify the user |

## 17. Open items

| Item | Status |
|---|---|
| Meaning of "major update" for the Home layout reset | To be defined |
