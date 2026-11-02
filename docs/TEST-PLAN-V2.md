# DesignHub Version 2 Test Plan

| | |
|---|---|
| Document | DesignHub version 2 test plan |
| Version | 0.1 (draft for review) |
| Date | 2026-11-01 |
| Owner | Devon |
| Status | Draft |
| Related | [SPEC.md](SPEC.md), [DATA-MODEL.md](DATA-MODEL.md), [SCREENS.md](SCREENS.md), [TEST-PLAN.md](TEST-PLAN.md) |

## Contents

1. Scope
2. Approach
3. Test environment
4. Test data
5. Acceptance criteria
6. Checklist
7. Phone and tablet checks
8. End-to-end scenarios
9. Open items

Appendix A. Version 2 seed data additions

---

## 1. Scope

This plan defines what is tested before DesignHub version 2 is accepted and what "working" means for each version 2 feature in [SPEC.md](SPEC.md) section 16.2:

- Vendor and Contractor roles with company scoping and expiry
- Custom roles and editable workflows
- Sprints, milestones, timeline view and time tracking
- Templates, inline comments, PDF export
- Email notifications
- CSV import and export

Version 1 behavior is covered by [TEST-PLAN.md](TEST-PLAN.md) and is rechecked by the regression checks in section 6.17.

Not in scope: version 3 features, the work item dashboard, load and performance testing.

## 2. Approach

The approach, priorities, issue logging and run record are the same as [TEST-PLAN.md](TEST-PLAN.md) sections 2 and 6. Failed Should checks are logged as GitHub issues with label `v2-test`.

| Item | Decision |
|---|---|
| Tester | Devon |
| Order | Installation checks, then the checklist on seed data, then regression checks, then the scenarios on a fresh install, then backup and restore |
| Email checks | Run after the mail server is decided and configured. Until then section 6.14 is skipped. |

## 3. Test environment

As [TEST-PLAN.md](TEST-PLAN.md) section 3, with these changes:

| Item | Value |
|---|---|
| Containers | `designhub`, `postgres`, `proxy`, `pdf` |
| Second browser | A private window or second browser, for checks that need a Vendor or Contractor signed in beside an internal user |
| PDF viewer | The browser's built-in PDF viewer |
| Spreadsheet | Any spreadsheet program that saves CSV as UTF-8, for import checks |

## 4. Test data

The version 1 seed data set (TEST-PLAN.md Appendix A) plus the additions in Appendix A of this plan. Seed data loads both.

## 5. Acceptance criteria

| Criterion | Requirement |
|---|---|
| Must checks | All pass |
| Should checks | Failures logged as `v2-test` issues |
| Regression | All section 6.17 checks pass |
| Access | No check in sections 6.2 to 6.4 shows a Vendor or Contractor any record outside their scope, in any screen, export, PDF, search result or notification |
| Scenarios | SCN-10 to SCN-14 complete without workarounds |

## 6. Checklist

"Signed in as" names the user who performs the check. Seed names are in Appendix A of this plan and of TEST-PLAN.md.

### 6.1 Installation and upgrade (IN2)

| ID | Check | Expected | Priority |
|---|---|---|---|
| IN2-01 | On the VM running version 1 with data, pull the version 2 images and start the stack | All four containers healthy. Version 1 data unchanged. Audit log records the schema upgrade. | Must |
| IN2-02 | Super Admin opens Admin, System | Database and PDF service status shown as connected | Must |
| IN2-03 | Block internet access and repeat a PDF export and a CSV import | Both work | Must |
| IN2-04 | Fresh install, load seed data | Appendix A records exist in addition to the version 1 seed data | Must |
| IN2-05 | Open Admin, Workflows after upgrade | Each type shows the version 1 workflow as version 1. Existing items keep their states. | Must |

### 6.2 Companies (CMP)

| ID | Check | Expected | Priority |
|---|---|---|---|
| CMP-01 | `editor1` creates a company | Next ID COMP-0003 offered. Saved with kind and status. | Must |
| CMP-02 | `editor1` adds 100001-001 to COMP-0001 Supplied items | Part shows COMP-0001 under Suppliers. No change order needed. Audit log records the link. | Must |
| CMP-03 | Open 100002-001, supplied by COMP-0002 (Conditional) | Amber banner shows the supplier status | Must |
| CMP-04 | `viewer1` opens a company | Read only | Must |
| CMP-05 | Deactivate COMP-0002 | It cannot be picked for new links. `vendor2` can no longer sign in. | Must |
| CMP-06 | Try to create a company with an existing name | Refused with a message | Should |

### 6.3 Vendors (VND)

| ID | Check | Expected | Priority |
|---|---|---|---|
| VND-01 | Sign in as `vendor1` | Vendor sidebar: Home, Supplied items, NCRs, Notifications, About. Company name in the top bar. | Must |
| VND-02 | Open Supplied items | Only items supplied by COMP-0001, released revisions only | Must |
| VND-03 | Open 100001-001 Files | Drawing visible and downloadable. Native design files hidden. | Must |
| VND-04 | Set "Vendors may download native files" Yes on 100001-001 as `editor1`, recheck as `vendor1` | Native design files now listed | Must |
| VND-05 | Type the ID of an item not supplied by COMP-0001, a page ID and a change order ID in search and in the address bar | Each shows Not found | Must |
| VND-06 | Open NCR-0101 (Supplier COMP-0001) | Report read only, Supplier response editable. Saving sets Supplier responded on and notifies the NCR assignee. | Must |
| VND-07 | Open NCR-0102 (Supplier COMP-0002) by ID | Not found | Must |
| VND-08 | `editor1` adds an Internal comment and a normal comment on NCR-0101 | `vendor1` sees only the normal comment | Must |
| VND-09 | `vendor1` uploads a supplier document on 100001-001 | Listed under Supplier documents. Released revision files unchanged. | Must |
| VND-10 | `vendor1` exports Supplied items to CSV and NCR-0101 to PDF | Only in-scope rows. PDF has no Internal comments. | Must |
| VND-11 | Notifications for out-of-scope records | `vendor1` receives none, including when watching is attempted | Must |

### 6.4 Contractors and account expiry (CTR)

| ID | Check | Expected | Priority |
|---|---|---|---|
| CTR-01 | Sign in as `contractor1` (member of SP-0001 only) | Only SP-0001 in the space switcher | Must |
| CTR-02 | Create and edit a page and a task in SP-0001 | Allowed | Must |
| CTR-03 | Open a page or work item ID from SP-0002 | Not found | Must |
| CTR-04 | Open the Library | Only released items with home space SP-0001, read only. No change orders entry. | Must |
| CTR-05 | Set `contractor1` expiry to tomorrow as `admin1` | Expiry banner shown to `contractor1`. `admin1` notified. | Must |
| CTR-06 | Set expiry to yesterday | Next request signs `contractor1` out. Sign-in refused with the expiry message. | Must |
| CTR-07 | Extend the expiry | Sign-in works again. Audit log records both changes. | Must |
| CTR-08 | Try to save a Vendor or Contractor account without company or expiry | Refused with a message | Must |

### 6.5 Custom roles (RL2)

| ID | Check | Expected | Priority |
|---|---|---|---|
| RL2-01 | `admin1` copies Editor as "Designer" and removes Open change orders | New column in the grid. Built-in columns read only. | Must |
| RL2-02 | Give `editor2` the Designer library role | `editor2` can edit drafts but has no New change order action | Must |
| RL2-03 | Try to copy Super Admin | Not offered | Must |
| RL2-04 | Deactivate Designer while `editor2` holds it | Deactivated, not deleted. Not offered for other users. `editor2` keeps it. | Should |
| RL2-05 | Copy Vendor as "Vendor, read only" without Supplier response editing | Company scoping kept. A user with it sees the same records as `vendor1` but cannot edit the response. | Must |
| RL2-06 | Audit log | Every role change recorded with before and after | Must |

### 6.6 Workflows (WFL)

| ID | Check | Expected | Priority |
|---|---|---|---|
| WFL-01 | `admin1` adds state "Waiting on supplier" (Active) to the Task workflow between In Progress and In Review, with transitions | Task board shows the new column in order | Must |
| WFL-02 | Allow In Review to Done only for space Admins | `editor2` does not see Done in the status dropdown of a task In Review. `admin1` does. | Must |
| WFL-03 | Make Due date required to enter In Progress | Moving a task with no due date asks for it | Must |
| WFL-04 | Remove "Waiting on supplier" while TSK-0101 is in it | Save asks for a new state for TSK-0101. Its history records the move. | Must |
| WFL-05 | Try to remove the NCR Disposition state | Not allowed; lock icon shown | Must |
| WFL-06 | Rename NCR state Verified to "Effective" | NCR boards and views show the new name. Root cause rule still enforced. | Must |
| WFL-07 | Move an NCR with Requires design change Yes and no change order to a Done state | Refused with the change order message | Must |
| WFL-08 | Rename a task state, then open My Work and an epic's progress | Both still correct (categories) | Must |
| WFL-09 | Admin, Workflows history | Earlier versions listed with who and when | Should |
| WFL-10 | Change order workflow | Not listed in Admin, Workflows | Must |

### 6.7 Sprints (SPR)

| ID | Check | Expected | Priority |
|---|---|---|---|
| SPR-01 | `editor1` creates SPR-0003 in SP-0001 | Listed as Planned | Must |
| SPR-02 | Drag two backlog items onto SPR-0003 | Items show the sprint | Must |
| SPR-03 | Start SPR-0003 while SPR-0001 is Active | Refused: one active sprint per space | Must |
| SPR-04 | Close SPR-0001, move unfinished items to the next Planned sprint | Unfinished items now in SPR-0002. Sprint view shows committed and completed. | Must |
| SPR-05 | Open the Board | Defaults to the active sprint. Picker shows other sprints, No sprint and All. | Must |
| SPR-06 | Sprint start and close notify sprint item assignees | Notifications received | Should |

### 6.8 Milestones (MST)

| ID | Check | Expected | Priority |
|---|---|---|---|
| MST-01 | Open MS-0001 | Progress matches Appendix A expected values by count and by estimate | Must |
| MST-02 | MS-0002 (target D+5, under 80 percent done) | Health At risk, amber. Owner notified once. | Must |
| MST-03 | Move a target date into the past | Health Late, red | Must |
| MST-04 | Complete all linked items | Health Done, Completed on set | Must |
| MST-05 | Space home | Upcoming milestones with health | Should |

### 6.9 Timeline (TML)

| ID | Check | Expected | Priority |
|---|---|---|---|
| TML-01 | Open Timeline in SP-0001 | Epics as bars grouped by epic, milestones as diamonds, active sprint band | Must |
| TML-02 | Group by milestone | Rows regroup | Must |
| TML-03 | Drag a task's due date end as `editor1` | Due date updated, history records it. Ctrl+Z restores it. | Must |
| TML-04 | Same as `viewer1` | Bars cannot be dragged | Must |
| TML-05 | `blocks` link between TSK-0101 and TSK-0102 | Arrow drawn | Should |
| TML-06 | Week, Month, Quarter | Scale changes; today line shown | Should |

### 6.10 Time tracking (TIM)

| ID | Check | Expected | Priority |
|---|---|---|---|
| TIM-01 | `contrib1` logs 1.5 h on TSK-0101 | Entry listed. Time logged and Remaining update. | Must |
| TIM-02 | Log 0.1 h, 25 h, and a future date | Each refused with a message | Must |
| TIM-03 | `contrib1` edits own entry; tries to edit `editor1`'s entry | Own allowed; other refused. `admin1` can edit both. | Must |
| TIM-04 | `viewer1` and `vendor1` | No Log time action | Must |
| TIM-05 | Epic EPC-0101 | Totals of its child items | Must |
| TIM-06 | Time report for last 7 days grouped by person | Totals match Appendix A | Must |
| TIM-07 | Export the time report to CSV | Same rows and totals | Must |
| TIM-08 | My time | Week view with day and item totals | Should |

### 6.11 Templates (TPL)

| ID | Check | Expected | Priority |
|---|---|---|---|
| TPL-01 | New page in SP-0001 | Picker lists Blank page and the six initial templates | Must |
| TPL-02 | Create a page from Meeting notes | Page type Meeting notes. {date}, {user}, {space} filled. Title from the pattern. | Must |
| TPL-03 | `admin1` creates a template scoped to SP-0002 | Offered in SP-0002 only | Must |
| TPL-04 | Edit the Meeting notes template | Existing pages unchanged; new pages use the new body | Must |
| TPL-05 | Template with a live Bill of Design block | New page shows the live block | Should |
| TPL-06 | Deactivate a template | No longer offered | Must |

### 6.12 Inline comments (ICM)

| ID | Check | Expected | Priority |
|---|---|---|---|
| ICM-01 | `editor2` selects text on PG-0003 and comments | Text underlined. Comment in the panel with its quote. | Must |
| ICM-02 | `editor1` replies and @mentions `editor2` | Reply shown; `editor2` notified | Must |
| ICM-03 | Edit PG-0003 so the quoted text is removed, save | Comment shows Text changed and keeps its quote | Must |
| ICM-04 | Resolve the comment | Highlight removed; comment under Resolved | Must |
| ICM-05 | Inline comment on a requirement's description | Works as on pages | Should |
| ICM-06 | Phone: long-press selection and comment | Works | Should |

### 6.13 PDF export (PDF)

| ID | Check | Expected | Priority |
|---|---|---|---|
| PDF-01 | Export PG-0003 | Header with ID, title, revision and status. Footer with exporter, date, page N of M. Tables and live blocks match the screen. | Must |
| PDF-02 | Export PG-0001 with pages below it | One file, pages in tree order | Must |
| PDF-03 | Export a controlled page that is not released | "Not released" on every page | Must |
| PDF-04 | Export 100004-201 with Bill of Design | BOM matches the screen and the extended quantities | Must |
| PDF-05 | Export CO-0002 after release | Approvals and signatures with time and meaning | Must |
| PDF-06 | Export NCR-0101 | All stages, supplier response | Must |
| PDF-07 | Export a record with a locked BOM line as `editor2` | Locked line hidden as on screen | Must |
| PDF-08 | Audit log | Each export recorded | Must |
| PDF-09 | Print view | Browser print dialog with the same layout | Should |

### 6.14 Email notifications (EML)

Run after the mail server is configured.

| ID | Check | Expected | Priority |
|---|---|---|---|
| EML-01 | Super Admin sends a test email | Received | Must |
| EML-02 | `editor2` sets email delivery Immediately for Assigned; `editor1` assigns a task to `editor2` | Email received with event, ID, title and link only | Must |
| EML-03 | Daily summary at a set hour | One email listing the day's notifications | Must |
| EML-04 | Delivery Off | No email | Must |
| EML-05 | Event about a record `vendor1` cannot see | No email to `vendor1` | Must |
| EML-06 | Mail server unavailable | In-app notifications still delivered; the failure is shown in Admin, System | Must |

### 6.15 CSV export (CSX)

| ID | Check | Expected | Priority |
|---|---|---|---|
| CSX-01 | Export each Library tab with a filter and chosen columns | File has those rows and columns in that order | Must |
| CSX-02 | Export BOM, where-used, traceability, change orders, work List, NCR List, sprints, milestones, time report, search results, Admin users and audit log | Each file matches its table | Must |
| CSX-03 | Full export of parts | Every part field, one row per part | Must |
| CSX-04 | Export as `editor2` with locked records present | Locked records absent | Must |
| CSX-05 | Open an export with accented and comma-containing values in a spreadsheet | Values intact | Must |
| CSX-06 | Audit log | Each export recorded | Must |

### 6.16 CSV import (CSI)

Files are in Appendix A.3.

| ID | Check | Expected | Priority |
|---|---|---|---|
| CSI-01 | Download a blank parts file | Header row matches the export column names | Must |
| CSI-02 | Import `parts-good.csv` (create only) | Preview shows 5 Create. After import, 5 Draft parts with next numbers where blank. | Must |
| CSI-03 | Import `parts-bad.csv` | Preview shows each error with row and reason. Import disabled. Downloaded error file has the error column. | Must |
| CSI-04 | Import an update to a released part | Error "Released items change only through a change order." | Must |
| CSI-05 | Export parts, change a draft's name, import with Create and update | 1 Update, 0 Create | Must |
| CSI-06 | Import `bom-lines.csv` into draft 100004-201, Add to BOM | Lines added; validation runs | Must |
| CSI-07 | Same file with Replace BOM | BOM equals the file | Must |
| CSI-08 | Import BOM lines into a released assembly | Not offered | Must |
| CSI-09 | Import `work-items.csv` into SP-0001 | Items created in the workflow's initial state | Must |
| CSI-10 | Import requirements, manufacturers and companies | Created as described | Must |
| CSI-11 | `viewer1` and `contrib1` | No Import CSV action | Must |
| CSI-12 | Admin, Imports | Every import listed with its file | Must |

### 6.17 Regression (REG)

| ID | Check | Expected | Priority |
|---|---|---|---|
| REG-01 | Rerun all Must checks in TEST-PLAN.md sections 7.3, 7.5 to 7.9 and 7.13 | All pass | Must |
| REG-02 | Rerun SCN-02 and SCN-03 from TEST-PLAN.md | Pass | Must |
| REG-03 | Backup and restore after the version 2 checks | Companies, roles, workflows, sprints, milestones, time entries, templates and imports restored | Must |

## 7. Phone and tablet checks

| ID | Screen | Check | Phone expected | Tablet expected |
|---|---|---|---|---|
| RS2-01 | Work item | Log time on TSK-0101 | Form usable | Same |
| RS2-02 | Timeline | Open Timeline | List grouped by month | Bars shown |
| RS2-03 | Vendor NCR | `vendor1` saves a supplier response | Usable | Same |
| RS2-04 | Inline comment | Comment on PG-0003 text | Long-press works | Same |
| RS2-05 | PDF | Export NCR-0101 | File downloads and opens | Same |
| RS2-06 | Sprints | Open Sprints and the backlog | "Add to sprint" usable without drag | Drag works |

## 8. End-to-end scenarios

### SCN-10 Supplier nonconformance

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `contrib1` | Report an NCR on 100001-001 with Supplier COMP-0001 | NCR created |
| 2 | `editor1` | Request supplier response, due in 7 days | `vendor1` notified |
| 3 | `vendor1` | Enter root cause and corrective action, upload a document | Responded on set; `editor1` notified |
| 4 | `editor1`, `appr1` | Disposition Return to vendor with signature | Dispositioned |
| 5 | `editor1` | Close with no design change | Closed. `vendor1` still sees it, read only. |

### SCN-11 Contractor build-out sprint

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `admin1` | Create a contractor account for COMP-0002 with expiry D+30, member of SP-0001 | Account works |
| 2 | `editor1` | Plan a sprint with five tasks, two assigned to the contractor, with estimates | Sprint Planned |
| 3 | `editor1` | Start the sprint | Board shows it |
| 4 | Contractor | Move tasks and log time | Allowed in SP-0001 only |
| 5 | `editor1` | Close the sprint; move unfinished items | Report shows committed against completed and estimate against logged |
| 6 | `editor1` | Export the time report | Matches the screen |

### SCN-12 Custom workflow for factory tasks

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `admin1` | Add "Waiting on supplier" and restrict Done to space Admins | Saved as a new version |
| 2 | `editor2` | Work a task through every state | Only allowed transitions offered |
| 3 | `admin1` | Remove the added state while an item is in it | Mapping step; history records the move |

### SCN-13 Design review package

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `editor1` | Create a page from Design review for 100004-201 with a live BOM block | Page created from template |
| 2 | `appr1`, `appr2` | Leave inline comments | Highlighted |
| 3 | `editor1` | Resolve and edit | Text changed comments shown correctly |
| 4 | `editor1` | Export the page and its children to PDF | One file matching the screen |

### SCN-14 Spreadsheet migration

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `editor1` | Import Devon's existing part list | Drafts created, errors fixed in the file and reimported |
| 2 | `editor1` | Import a BOM into a draft assembly | Flat BOM totals match the spreadsheet |
| 3 | `editor1` | Export the Library and compare with the original | Same rows and values |

## 9. Open items

| Item | Affects |
|---|---|
| Mail server | Section 6.14 |
| Devon's existing part list and BOM for SCN-14 | Section 8 |
| CSV import record types (confirm) | Section 6.16 |

---

## Appendix A. Version 2 seed data additions

### A.1 Companies and users

| ID | Name | Kind | Supplier status | Supplies |
|---|---|---|---|---|
| COMP-0001 | Northfield Fabrication | Vendor | Approved | 100001-001, 100003-201 |
| COMP-0002 | Ridgeway Electrical | Vendor, Contractor | Conditional | 100002-001 |

| Username | Library role | Space role | Company | Expiry |
|---|---|---|---|---|
| `vendor1` | Vendor | | COMP-0001 | D+90 |
| `vendor2` | Vendor | | COMP-0002 | D+90 |
| `contractor1` | Contractor | SP-0001 Contractor | COMP-0002 | D+30 |

### A.2 Planning and time

| Record | Details |
|---|---|
| TSK-0101 to TSK-0103 | Tasks in SP-0001 assigned to `contrib1`, `editor1` and `editor2`; TSK-0101 Done, the others Open |
| SPR-0001 | SP-0001, Active, D-7 to D+7, items TSK-0101, TSK-0102, TSK-0103 |
| SPR-0002 | SP-0001, Planned, D+8 to D+21 |
| MS-0001 | SP-0001, target D+30, owner `editor1`, items TSK-0101 (Done, 4 h estimate), TSK-0102 (Open, 4 h), TSK-0103 (Open, 2 h). Expected progress 1 of 3 by count, 4 of 10 h by estimate. |
| MS-0002 | SP-0001, target D+5, items TSK-0102, TSK-0103. Expected health At risk. |
| EPC-0101 | Epic over TSK-0101 to TSK-0103 |
| NCR-0101 | SP-0001, found on 100001-001 rev 01, Supplier COMP-0001, Reported |
| NCR-0102 | SP-0001, found on 100002-001 rev 01, Supplier COMP-0002, Reported |
| Time entries | `contrib1` 3 h on TSK-0101 (D-2); `editor1` 2 h on TSK-0102 (D-1); `editor2` 1 h on TSK-0103 (D-1). Expected last-7-days totals: `contrib1` 3, `editor1` 2, `editor2` 1. |

### A.3 Import files

Loaded with the seed data into the file store and offered in Admin, Imports, Sample files.

| File | Content |
|---|---|
| `parts-good.csv` | 5 production parts, 2 with blank numbers |
| `parts-bad.csv` | 4 rows: a used number, a missing name, an unknown unit, an inactive manufacturer |
| `bom-lines.csv` | 4 BOM lines for 100004-201 |
| `work-items.csv` | 3 tasks and 1 issue for SP-0001 |
