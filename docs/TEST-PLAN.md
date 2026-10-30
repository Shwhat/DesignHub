# DesignHub Version 1 Test Plan

| | |
|---|---|
| Document | DesignHub version 1 test plan |
| Version | 0.2 (draft for review) |
| Date | 2026-10-30 |
| Owner | Devon |
| Status | Draft |
| Related | [SPEC.md](SPEC.md), [DATA-MODEL.md](DATA-MODEL.md), [SCREENS.md](SCREENS.md) |

## Contents

1. Scope
2. Approach
3. Test environment
4. Test data
5. Acceptance criteria
6. Recording results and logging issues
7. Checklist
8. Phone and tablet checks
9. End-to-end scenarios
10. Prototype acceptance checks
11. Open items

Appendix A. Seed data set
Appendix B. Realistic sample

---

## 1. Scope

This plan defines what is tested before DesignHub version 1 is accepted and what "working" means for each feature. It covers every version 1 feature in [SPEC.md](SPEC.md) section 16.1.

Not in scope: version 2 and version 3 features, Google sign-in, email notifications, InfluxDB, load and performance testing, and the Home layout reset after a major update.

## 2. Approach

| Item | Decision |
|---|---|
| Tester | Devon |
| Method | Manual testing in the browser, plus a small number of command-line checks on the VM |
| Format | A checklist per screen and feature (section 7), and end-to-end scenarios (section 9) |
| Test data | Seed data set for the checklist; realistic sample for the scenarios |
| Phone and tablet | Key screens only (section 8) |
| Order | Installation checks, then the checklist on seed data, then the scenarios on a fresh install with the realistic sample, then backup and restore |

Each check has a priority:

| Priority | Meaning |
|---|---|
| Must | Required for version 1 acceptance |
| Should | Expected in version 1. A failure is logged as an issue and does not block acceptance. |

## 3. Test environment

| Item | Value |
|---|---|
| Host | Proxmox VM with Docker and Docker Compose installed. DesignHub installed from the published `docker-compose.yml`. |
| Network | Local network, fixed IP address |
| Address | The VM's fixed IP address, through the `proxy` container |
| Containers | `designhub`, `postgres`, `proxy`. `influxdb` is not started in version 1. |
| Desktop browser | Current Chrome or Edge on a computer on the local network |
| Phone | Devon's phone, default browser |
| Tablet | A tablet, or desktop browser device emulation at 768 x 1024 |
| Internet | Checks are run once with the VM's internet access available and once with it blocked at the firewall (check INS-08) |

## 4. Test data

### 4.1 Seed data set

- Defined in Appendix A.
- Loaded only when setup is run with the test data option. A production install never contains seed data.
- Dates in the seed data are relative to the load date, so overdue and due-soon checks give the same result on any day.
- All seed user accounts use one password, entered when the seed data is loaded.
- The checklist (section 7) gives the exact expected result for each check against the seed data.

### 4.2 Realistic sample

- Defined in Appendix B. Devon supplies the real items before the scenarios are run.
- Entered by hand through the user interface on a fresh install, as part of scenario SCN-01. CSV import is a version 2 feature.
- Expected BOM totals come from Devon's own reference BOM for the panel.

## 5. Acceptance criteria

Version 1 is accepted when, in one complete test run on a fresh install:

1. Every Must check in section 7 passes.
2. Every Must check in section 8 passes on phone and on tablet.
3. Every scenario in section 9 completes with the stated results.
4. Every adapted prototype check in section 10 passes.
5. Every failed Should check is logged as an issue.

A Must failure fixed during a test run is retested, and every other check in the same area is run again.

## 6. Recording results and logging issues

### 6.1 Test run record

- Each test run has a record file: `docs/test-runs/YYYY-MM-DD.md`.
- The record lists the build tested (image tag and git commit) and every check ID with one result.

| Result | Meaning |
|---|---|
| Pass | Behaves as stated in the Expected column |
| Fail | Does not behave as stated. Issue number recorded. |
| Blocked | Cannot be run because of another failure. Blocking check ID recorded. |
| Not run | Not run in this test run |

### 6.2 Issues

- Failures are logged as GitHub issues in `Shwhat/DesignHub` with the label `v1-test`.
- Title format: `[CHECK-ID] short description`, for example `[ASM-03] Extended quantity wrong for nested assembly`.
- Each issue states the steps, the expected result, the actual result, the priority of the check (Must or Should) and a screenshot where useful.

## 7. Checklist

Seed user names (`editor1`, `appr2` and so on), records and expected values are defined in Appendix A. "Signed in as" names the user who performs the check.

### 7.1 Installation (INS)

| ID | Check | Expected | Priority |
|---|---|---|---|
| INS-01 | Start the stack with the single start command on the VM | `designhub`, `postgres` and `proxy` are running and report healthy | Must |
| INS-02 | First start on an empty VM | Database password and self-signed certificate generated. PostgreSQL database and user created and connected with no manual database steps. | Must |
| INS-03 | First start on an empty VM | Engineering repository exists on the `repo` volume. `git log` shows an initial commit. | Must |
| INS-04 | Run the setup command on the VM, enter a Super Admin username and password, then sign in | Password asked in the terminal and not echoed. Sign-in succeeds and Home opens. | Must |
| INS-05 | Open the fixed IP address from another computer and from the phone | Sign-in screen loads | Must |
| INS-06 | Load seed data with the test data option | Records in Appendix A exist. Library validation reports no errors. | Must |
| INS-07 | Stop and start the stack with the single stop and start commands | All data is unchanged | Must |
| INS-08 | Block the VM's internet access at the firewall, restart the stack and repeat INS-04 and a sample of section 7 checks | Behavior is unchanged. The browser network panel shows requests only to the fixed IP address. | Must |
| INS-09 | Reboot the VM | The stack starts automatically and all data is unchanged | Should |
| INS-10 | Super Admin opens Admin, System | Database status shows PostgreSQL connected | Should |

### 7.2 Sign-in and sessions (SGN)

| ID | Check | Expected | Priority |
|---|---|---|---|
| SGN-01 | Sign in as `editor1` with the correct password | Home opens | Must |
| SGN-02 | Sign in as `editor1` with a wrong password | "Username or password is incorrect." | Must |
| SGN-03 | Sign in with an unknown username | Same message as SGN-02 | Must |
| SGN-04 | Enter a wrong password repeatedly for `editor2` | Rate-limit message appears. The correct password is refused until the limit period ends. | Must |
| SGN-05 | Admin sets a sign-in message, then clears it | Message shows above the form, then is hidden | Should |
| SGN-06 | Click "Forgot password" | "Contact your administrator." | Should |
| SGN-07 | Sign in without "Keep me signed in" and leave the browser idle for more than 30 minutes | Next action returns to the sign-in screen | Must |
| SGN-08 | Sign in with "Keep me signed in", close and reopen the browser | Still signed in. Signed out once 1 day has passed. | Must |
| SGN-10 | Sign out, then press the browser Back button | Sign-in screen. No record content shown. | Must |
| SGN-11 | Sign in as `editor1` on desktop and phone. On desktop, Settings, Security, sign out of other devices. | Phone session ends at its next action | Should |
| SGN-12 | Admin disables `viewer1`, then `viewer1` tries to sign in | Sign-in refused with the standard message | Must |
| SGN-13 | `editor1` changes password | Old password refused, new password accepted | Must |
| SGN-14 | Admin resets the password of `contrib1` | `contrib1` signs in with the new password | Must |

### 7.3 Roles and access (ROL)

| ID | Check | Expected | Priority |
|---|---|---|---|
| ROL-01 | Signed in as `viewer1`, open PG-0002, 100003-201 and TSK-0001 | All readable. No Edit, Create or status actions shown. | Must |
| ROL-02 | Signed in as `contrib1`, create a task and an NCR in SP-0001, then open 100001-001 | Task and NCR created. Part is read-only. | Must |
| ROL-03 | Signed in as `editor2`, create a part, edit a page and open a change order | All succeed | Must |
| ROL-04 | Signed in as `appr1` and as `editor2`, open CO-0002 | `appr1` sees the decision with their recorded approval. `editor2` sees no decision bar. | Must |
| ROL-05 | Signed in as `appr1`, open an Approved change order | No Release action | Must |
| ROL-06 | Signed in as `admin1`, open Admin, Users | Can edit users except `superadmin`. Super Admin cannot be assigned. | Must |
| ROL-07 | Signed in as `admin1` | System settings, backups and database status are not shown | Must |
| ROL-08 | Signed in as `editor2` (no role in SP-0002), open the space switcher, search "Concept", and open the URL of PG-0006 | SP-0002 not listed. No results. URL shows not found. | Must |
| ROL-09 | Signed in as `editor2`, look for DOC-0002 in the Library, search and by its URL | Not listed, not found in search, URL shows not found | Must |
| ROL-10 | Signed in as `editor2`, open the BOM of 100004-201 | The 100007-001 line is not shown. Validation results match those seen by `editor1`. | Must |
| ROL-11 | Admin changes the library role of `contrib1` | Change appears in the audit log with user, time, old and new role | Must |
| ROL-12 | Admin opens Roles | Read-only grid, roles as columns, permissions as rows | Should |
| ROL-13 | Admin turns off "Can add or remove change order approvers" for Editor | `editor1` no longer sees "Add approver" on CO-0003 | Should |

### 7.4 Identification and numbering (NUM)

| ID | Check | Expected | Priority |
|---|---|---|---|
| NUM-01 | Signed in as `editor1`, create a production part | Form offers 100008-001 | Must |
| NUM-02 | Change the offered suffix to -201 and create | Assembly 100008-201 created | Must |
| NUM-03 | Create a production part with number 100001-001 | Refused: number already used | Must |
| NUM-04 | Create a production part with number 100001-003 | Created | Must |
| NUM-05 | Create a production part with number 100010-001, then create two parts with the offered numbers | Offers are 100009-001 then 100011-001 | Must |
| NUM-06 | Open the create form, then cancel. Open it again. | Same number offered both times | Must |
| NUM-07 | Delete the never-released draft 100001-002. Then create a part with number 100001-002. | Deletion succeeds. Number refused as retired. | Must |
| NUM-08 | Open released 100001-001 | No Delete action | Must |
| NUM-09 | Create a reference part | Offers 1000002 | Must |
| NUM-10 | Convert reference part 1000001 to production | New production number issued. 1000001 shows a "replaced by" link to it. | Must |
| NUM-11 | Create a COTS part with manufacturer PXC and order number 1234567 | ID is PXC.1234567. Order number field holds 1234567. | Must |
| NUM-12 | Create the same COTS part again | Refused: number already used | Must |
| NUM-13 | Admin deactivates manufacturer AB, then create a COTS part | AB not offered | Should |
| NUM-14 | Admin edits the code of manufacturer PXC | Refused: code in use | Should |
| NUM-15 | Create a requirement | ID is REQ-0004 | Must |
| NUM-16 | Edit an item and try to change its item number | Not editable, or refused on save | Must |

### 7.5 Library and engineering items (ITM)

| ID | Check | Expected | Priority |
|---|---|---|---|
| ITM-01 | Open the Library | Tabs: Parts, Assemblies, Requirements, Procedures, Documents. Part columns: ID, revision, name, status, owner, home space, part kind. | Must |
| ITM-02 | Parts tab, toggle to include Superseded and Obsolete | Default shows current revisions only. Toggle adds older revisions once they exist. | Must |
| ITM-03 | Choose and reorder columns, sign out and in | Column choice kept | Should |
| ITM-04 | Save a filter (status Draft), sign out and in | Filter available to `editor1` only | Should |
| ITM-05 | Open 100003-201 | Header: ID, rev 01, Released, name, work item badge showing 2 | Must |
| ITM-06 | Open 100005-001 | Status Draft, no revision shown | Must |
| ITM-07 | Edit 100005-001, clear Name, save | Save refused, Name marked required | Must |
| ITM-08 | Enter a Name longer than 50 characters | Refused or limited to 50 | Should |
| ITM-09 | Edit 100005-001, change Name, close the browser without saving, reopen and open the item | Unsaved draft restored for `editor1`. `editor2` sees the last saved version. | Must |
| ITM-10 | Save 100005-001 with a comment | History tab shows the new version with user, time and comment | Must |
| ITM-11 | `editor1` and `editor2` open 100005-001 for editing. `editor1` saves, then `editor2` saves. | `editor2`'s save is refused and differences are shown. `editor1`'s save is kept. | Must |
| ITM-12 | Open 100001-001 (Released) | Edit is not available. "Start new revision" creates rev 02 Draft. Rev 01 remains the released revision. | Must |
| ITM-13 | Upload a PDF and a .zip to the Files tab of 100005-001 with role Other | PDF previews in the browser. Zip downloads only. | Must |
| ITM-14 | Open the Files tab of released 100001-001 rev 01 | No upload, replace or remove actions | Must |
| ITM-15 | Add a "satisfies" link from 100005-001 to REQ-0003 | Link shown on 100005-001. "Satisfied by" shown on REQ-0003. | Must |
| ITM-16 | Add a link pinned to 100002-001 rev 01 | Link shows rev 01 after rev 02 is released | Should |
| ITM-17 | Open REQ-0001, REQ-0002 and REQ-0003 | Coverage: REQ-0001 complete. REQ-0002 "No test". REQ-0003 no design and "No test". | Must |
| ITM-18 | Edit PRC-0001 draft revision, drag step 3 to position 1 | Steps renumber 1, 2, 3 | Must |
| ITM-19 | View History of PG-0003 or 100005-001 and compare two versions | Added and removed lines highlighted | Must |
| ITM-20 | Signed in as `editor1`, Watch 100005-001. `editor2` edits and saves it. | `editor1` receives a "Watched record changed" notification | Must |

### 7.6 Assemblies and Bill of Design (ASM)

| ID | Check | Expected | Priority |
|---|---|---|---|
| ASM-01 | Open the BOM tab of 100004-201 | Opens on Tree | Must |
| ASM-02 | Expand the tree | 100003-201 shows its three lines. 100001-001 appears under 100004-201 and under 100003-201. | Must |
| ASM-03 | Read extended quantities in the tree | PXC.3031212: 30. SIE.3RT2015-1BB41: 6. 100001-001 under 100003-201: 6. | Must |
| ASM-04 | Switch to Flat | Each unique part once. 100001-001: 10. PXC.3031212: 30. SIE.3RT2015-1BB41: 6. RIT.8606000: 1. | Must |
| ASM-05 | Open Where-used on 100001-001 | Lists 100003-201 and 100004-201 | Must |
| ASM-06 | Start rev 02 of 100003-201 and add a line by typing part of an ID | Search-as-you-type finds the item. Find number defaults to 40. | Must |
| ASM-07 | Drag a line to a new position | Find numbers follow the new order | Should |
| ASM-08 | Add a line with quantity 2.5 | Accepted and saved with the decimal quantity | Must |
| ASM-09 | Set a line with quantity 3 and reference designators K1 K2 | Warning shown. Save is allowed. | Must |
| ASM-10 | Add 100004-201 as a line in 100003-201 rev 02 | Cycle reported as a validation error. Page stays responsive. | Must |
| ASM-11 | Add 100006-901 as a BOM line | Refused | Must |
| ASM-12 | Open the Validation tab after ASM-09 and ASM-10 | Error and warning listed. Icons shown on the affected BOM lines. | Must |
| ASM-13 | Collapse part of the tree, leave and return | Tree expansion remembered | Should |
| ASM-14 | Unit column on BOM lines | Matches each child item's unit of measure | Should |

### 7.7 Change orders (CO)

| ID | Check | Expected | Priority |
|---|---|---|---|
| CO-01 | From 100005-001, Actions, Add to change order, CO-0003 | Listed in Affected items with action Release, from revision blank | Must |
| CO-02 | Edit 100005-001 under CO-0003 as `editor2`. Open 100005-001 outside the change order as `editor1`. | `editor1` sees the version before the change order edits | Must |
| CO-03 | Submit CO-0003 for review with no approvers | Refused: approvers required | Must |
| CO-04 | Add only `editor2` (the author) as approver and submit | Refused: author cannot be the only approver | Must |
| CO-05 | Open the Changes tab of CO-0002 | Side-by-side and single-column views both show the 100002-001 change | Must |
| CO-06 | Create a change order releasing 100004-201 without 100005-001 and submit with approvers `appr1` and `appr2` | Validation error for the unreleased child. Approve is refused while the error exists. | Must |
| CO-07 | Release 100005-001 on a change order with no Drawing file | Release warning for the missing Drawing. Approval and release are allowed. | Must |
| CO-08 | Open CO-0002 as `appr2` and scroll | Approve and Reject bar stays visible at the bottom | Must |
| CO-09 | `appr2` approves CO-0002 with a wrong password | Refused | Must |
| CO-10 | `appr2` approves CO-0002 with correct password and comment | Approvals tab shows decision, time and comment. CO-0002 state Approved, because every approver has now approved. | Must |
| CO-11 | Reject a change order in review | State Rejected. Decision, time and comment recorded. Affected items keep their status. | Must |
| CO-11a | Admin turns on "Return affected items to Draft", then reject a change order in review | Affected items return to Draft | Should |
| CO-12 | `ra1` releases CO-0002 | 100002-001 rev 02 Released, rev 01 Superseded. Released by and Released on filled. Baseline created. | Must |
| CO-13 | Open 100002-001 rev 01 | Banner: "You are viewing rev 01 (Superseded). Current: rev 02" with a link | Must |
| CO-14 | Open the CO-0001 baseline, then compare it with the CO-0002 baseline | Baseline shows data exactly as released. Comparison shows only the 100002-001 change. | Must |
| CO-15 | Obsolete PXC.3031212 through a change order | PXC.3031212 status Obsolete. Where-used shows it remains in 100003-201 rev 01. | Must |
| CO-16 | Cancel a change order in Draft | State Cancelled. Affected items keep their earlier status. | Should |
| CO-17 | Open the audit log | Approvals, rejections and releases from CO-09 to CO-15 listed | Must |
| CO-18 | On the VM, run `git log`, `git tag` and `git fsck --strict` on the engineering repository (a bare repository) | All succeed. One signed baseline tag per released change order. | Must |

### 7.8 Spaces and pages (PAG)

| ID | Check | Expected | Priority |
|---|---|---|---|
| PAG-01 | Signed in as `editor1`, open the space switcher | SP-0001 and SP-0002 listed | Must |
| PAG-02 | Enter SP-0001 | Page tree expands. Opening PG-0003 shows breadcrumbs: Factory Buildout, Control panel overview, Kickoff meeting notes. | Must |
| PAG-03 | Create a child page under PG-0002, rename it, move it by the "..." menu, then by drag | All succeed. Tree and breadcrumbs update. | Must |
| PAG-04 | Open the SP-0001 home | Description, pinned links, recent changes, open work item counts and open change orders shown | Must |
| PAG-05 | Pin a link as `editor1`, then try as `viewer1` | `editor1` can pin. `viewer1` cannot. | Should |
| PAG-06 | Edit PG-0002 in each of Visual, Split, Source and Preview toggle, switching modes without saving | Content identical in every mode. Source view text unchanged after switching back. | Must |
| PAG-07 | Set default editing mode to Split in Settings, then edit a page | Editor opens in Split | Should |
| PAG-08 | Paste an image and drag a PDF into the editor | Image shown in the page. PDF attached and linked. | Must |
| PAG-09 | Insert a Bill of Design block for 100004-201 using "/" and using the Insert block menu | Block shows the BOM and flattened parts with the ASM-03 and ASM-04 values | Must |
| PAG-10 | Remove the ASM-08 to ASM-10 changes from 100003-201 rev 02, change a quantity, release it, then reopen PG-0002 | Block shows the new quantity without editing the page | Must |
| PAG-11 | View the work list block on PG-0002 | Shows NCR-0001 | Must |
| PAG-12 | Open PG-0004 (controlled) | Edit is "Edit under change order". Choosing an existing or new change order opens the editor. | Must |
| PAG-13 | With a draft revision of PG-0004 in progress, open it as `viewer1` | Released rev 01 content shown | Must |
| PAG-14 | Change PG-0004 to uncontrolled without a change order | Refused | Should |
| PAG-15 | On PG-0003, restore an earlier version | Restore saved as a new version in History | Must |
| PAG-16 | On PG-0002, add a comment, reply, mention `@editor2`, resolve the comment, then hide it | Reply indented one level. `editor2` notified. Hide recorded in the audit log. | Must |
| PAG-17 | Type `<script>alert(1)</script>` in a page body and save | Shown as text. No alert. | Must |
| PAG-18 | Admin archives SP-0002 | Hidden from navigation. Pages read-only. | Should |
| PAG-19 | Admin creates a space | Space gets the next SP number and a home page | Must |

### 7.9 Work items and NCRs (WRK)

| ID | Check | Expected | Priority |
|---|---|---|---|
| WRK-01 | Create a task, issue, action and epic in SP-0001 | Each gets the next number of its type | Must |
| WRK-02 | Create an issue without Area or Severity | Refused, fields marked required | Must |
| WRK-03 | Open the status dropdown on TSK-0002 | Only allowed next states listed. Blocked and Cancelled available. | Must |
| WRK-04 | On the Board, drag TSK-0002 to In Progress, then drag a card to a disallowed column | First move succeeds. Disallowed move refused. | Must |
| WRK-05 | `editor1` and `editor2` view the Board. `editor1` moves a card. | Card moves on `editor2`'s screen without reloading | Must |
| WRK-06 | List view: filter by assignee, save the filter, choose columns | Filter and columns kept after sign out and in | Should |
| WRK-07 | Add swimlanes by assignee | Board groups by assignee | Should |
| WRK-08 | Open EPC-0001 | Progress 33% (1 of 3 Done) | Must |
| WRK-09 | Open TSK-0002 | Shows "blocked by TSK-0001" | Must |
| WRK-10 | Signed in as `editor1`, open My Work | TSK-0001 first (overdue), then TSK-0002 and TSK-0004. Grouping by type and space works. | Must |
| NCR-01 | Create an NCR found on 100003-201 | Found on pinned to rev 01. Required fields enforced. | Must |
| NCR-02 | Move NCR-0001 to Dispositioned without disposition | Refused | Must |
| NCR-03 | Set disposition Rework and sign as `appr1` | Signature pop-up as for change orders. NCR-0001 Dispositioned. | Must |
| NCR-04 | Move NCR-0001 to Verified without root cause | Refused | Must |
| NCR-05 | Move NCR-0002 (requires design change) to Closed with no linked change order | Refused | Must |
| NCR-06 | On NCR-0002, Actions, Create change order | New change order with 100002-001 as affected item, linked as "fixed by" | Must |
| NCR-07 | Open NCR-0001 in Reported | Later stage fields visible and empty | Should |
| NCR-08 | Open the NCR Board | Columns Reported, Dispositioned, Corrective Action, Verified, Closed | Must |

### 7.10 Search (SRC)

| ID | Check | Expected | Priority |
|---|---|---|---|
| SRC-01 | Type "panel" in the top bar | Quick results grouped by type | Must |
| SRC-02 | Type "CO-0002" and press Enter | CO-0002 opens | Must |
| SRC-03 | Search "24 VDC" | REQ-0001 found with the match highlighted in the snippet | Must |
| SRC-04 | On the Search page filter by space, type, status, author and label | Results narrow correctly for each filter | Must |
| SRC-05 | Save a search, sign out and in | Saved search available | Should |
| SRC-06 | Open 100003-201, Links tab | Backlinks from 100004-201, ISS-0001 and NCR-0001 shown | Must |
| SRC-07 | Admin triggers a full index rebuild, then repeat SRC-03 | Same result | Should |

### 7.11 Notifications (NTF)

| ID | Check | Expected | Priority |
|---|---|---|---|
| NTF-01 | Assign TSK-0004 to `editor2` | `editor2` receives "Assigned" | Must |
| NTF-02 | Signed in as `editor1` after seed load | Bell shows unread count. Inbox shows the mention from PG-0002. | Must |
| NTF-03 | Submit a change order with approver `appr1`, then approve it | `appr1` receives "Approval requested". The owner receives "Approval decided". | Must |
| NTF-04 | Signed in as `contrib1` | ACT-0001 "Due soon" notification present | Should |
| NTF-05 | Click a notification | Record opens. Notification marked read. | Must |
| NTF-06 | Use filters All, Unread, Mentions, Approvals, and mark read, mark unread, mark all read, dismiss | Each works as named | Should |
| NTF-07 | `editor1` comments on DOC-0002 mentioning `@editor2` | `editor2` receives no notification | Must |
| NTF-08 | `editor2` turns off "Status changed" in Settings, then TSK-0003 changes status | No notification | Should |

### 7.12 Admin, profile and settings (ADM)

| ID | Check | Expected | Priority |
|---|---|---|---|
| ADM-01 | Create a user with a library role and a space role | User signs in and sees only that space | Must |
| ADM-02 | Open Users | Columns: username, display name, email, library role, active, last sign-in. No delete action. | Must |
| ADM-03 | Add manufacturer code TST, then deactivate it | Usable for new COTS parts while active, not after | Must |
| ADM-04 | In Lists, add a unit of measure, reorder by drag, deactivate a value in use | Saved. Value in use can be deactivated but not deleted. | Should |
| ADM-05 | Set File roles "Expected on" Drawing for -9XX, then release a -9XX part with no drawing | Release warning shown | Should |
| ADM-06 | Open the Audit log | Sign-ins, permission changes, approvals, releases and settings changes from this test run listed | Must |
| ADM-07 | Signed in as `editor1`, open the Admin URL directly | Refused | Must |
| ADM-08 | Settings, Appearance: Light, Dark, Follow device | Theme changes. Follow device matches the operating system setting. | Must |
| ADM-09 | Settings, Home layout: turn a section off and move another | Home updates | Should |
| ADM-10 | Set time zone to a different zone | Displayed times shift accordingly | Should |
| ADM-12 | Signed in as `editor1`, open Home | Sections listed in SCREENS.md section 3.1 show seed data: TSK-0001, TSK-0002, mention notification | Must |

### 7.13 Security (SEC)

| ID | Check | Expected | Priority |
|---|---|---|---|
| SEC-01 | Super Admin sets Allowed addresses to the VM's IP address in Admin, System. Then send a request with a foreign Host header: `curl -k -H "Host: example.com" https://<ip>/` | Refused | Must |
| SEC-02 | Send a POST to a save endpoint without a CSRF token using a valid session cookie | Refused | Must |
| SEC-03 | Call the JSON API without signing in | 401, no data | Must |
| SEC-04 | Browser developer tools, session cookie | Secure and HttpOnly set | Must |
| SEC-05 | Response headers on any page | Content-Security-Policy present | Should |
| SEC-06 | Open the URL of an item that does not exist | Not found page | Must |
| SEC-07 | `docker compose exec designhub sh -c "command -v python3 python"` | Nothing found | Should |
| SEC-08 | Look through every screen in this checklist | Only DesignHub branding shown | Should |

### 7.14 Backup and recovery (BAK)

| ID | Check | Expected | Priority |
|---|---|---|---|
| BAK-01 | Wait for a scheduled backup | Backup files for PostgreSQL and the engineering repository written to the `backups` volume | Must |
| BAK-02 | Restore the backup onto a fresh stack following the documented restore procedure | Record counts, a sample of records, BOM totals and history match the original. `git fsck --strict` passes. | Must |
| BAK-03 | After BAK-02, repeat SRC-03 and SRC-06 | Same results, without a manual index rebuild | Must |

## 8. Phone and tablet checks

Each check is run on the phone and on the tablet. All are Must unless noted.

| ID | Screen | Check | Phone expected | Tablet expected |
|---|---|---|---|---|
| RSP-01 | Sign-in | Sign in as `appr2` | Single column, full width | Form centered |
| RSP-02 | Home | Open Home as `editor1` | Sections in one column | Sections in two columns |
| RSP-03 | Library | Open Parts | Horizontal scroll. Simple list toggle shows ID, name, status. | Full table |
| RSP-04 | Assembly view | Open the BOM of 100004-201, expand, switch to Flat | Tree and flat views readable, values match ASM-03 and ASM-04 | Same |
| RSP-05 | Change order | `appr2` reviews and approves a change order | Changes tab single-column. Decision bar visible. Signature pop-up usable. | Decision bar visible. Signature pop-up usable. |
| RSP-06 | Page view | Edit PG-0003 in each of the four modes and save | All modes work. Split shows markdown above preview. | All modes work |
| RSP-07 | Work board | Open Board | Opens as the list view | Board columns shown |
| RSP-08 | NCR view | Open NCR-0001 and add a comment | Comments open from the Comments button | Same |
| RSP-09 | Search | Search with a type filter | Filters behind a Filters button | Same |
| RSP-10 | Notifications | Open the bell, then View all | Drop-down and inbox usable | Same |
| RSP-11 | All key screens | Switch Light and Dark | Both themes readable on every screen above (Should) | Same (Should) |

## 9. End-to-end scenarios

Scenarios use the realistic sample (Appendix B) on a fresh install with seed users only. Each step lists who performs it and the result that must be true before moving on.

### SCN-01 Build the control panel

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `admin1` | Add any manufacturers in Appendix B missing from the list | Manufacturers available |
| 2 | `editor1` | Create every COTS part in Appendix B with a datasheet | IDs follow MFR.ORDERNUMBER |
| 3 | `editor1` | Create the production piece parts with drawings | Numbers offered in sequence |
| 4 | `editor1` | Create the subassembly and top assembly and enter their BOMs | Flat BOM totals match Devon's reference BOM line for line |
| 5 | `editor1` | Create the requirements and the test procedure, and link satisfies and verifies | Traceability view shows the coverage Devon expects |
| 6 | `editor1` | Check the Validation tab of the top assembly | No errors |

### SCN-02 Initial release

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `editor1` | Create a change order of type Initial release with every SCN-01 item, approvers `appr1` and `appr2` | Validation shows no errors. Warnings only for missing file roles, if any. |
| 2 | `appr1`, `appr2` | Review the Changes tab and approve with electronic signature | State Approved |
| 3 | `ra1` | Release | Every item rev 01 Released. Baseline created. |
| 4 | `editor1` | Insert a BOM block for the top assembly on a page in SP-0001 | Block matches the released BOM |
| 5 | Devon on the VM | `git log`, `git tag`, `git fsck --strict` | Release commit and baseline tag present, fsck passes |

### SCN-03 Nonconformance to design change

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `contrib1` | Report an NCR found on the subassembly at receiving inspection, requires design change Yes | NCR Reported, linked to subassembly rev 01 |
| 2 | `appr1` | Disposition Rework with signature | NCR Dispositioned |
| 3 | `editor1` | Create change order from the NCR | Change order prefilled and linked "fixed by" |
| 4 | `editor1` | Start rev 02 of the subassembly, change a quantity, save under the change order | Changes tab shows the BOM difference |
| 5 | `editor1` | Check Where-used of the subassembly | Top assembly listed |
| 6 | `appr1`, `appr2`, `ra1` | Approve and release | Subassembly rev 02 Released, rev 01 Superseded. Top assembly BOM extended quantities reflect rev 02. |
| 7 | `contrib1` | Enter root cause, corrective action and verification, close the NCR | NCR Closed |
| 8 | Each user | Check the inbox | Each step's approval requests, decisions, assignments and status changes received by the right people |

### SCN-04 Controlled documentation

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `editor1` | Create a controlled page "Panel build standard" in SP-0001 and release it through a change order | Page rev 01 Released |
| 2 | `editor2` | Edit it under a new change order | `viewer1` still sees rev 01 |
| 3 | `appr1`, `ra1` | Approve and release | `viewer1` sees rev 02. History compares 01 and 02. |

### SCN-05 Planning the build

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `editor1` | Create an epic for the panel build with at least four tasks linked to the top assembly, one blocking another | Top assembly header badge counts the tasks |
| 2 | `editor1`, `editor2` | Work the tasks across the Board to Done | Epic progress updates at each step. Blocked task shows its blocker. |
| 3 | `editor2` | Check My Work | Overdue first, groups as chosen |

### SCN-06 Concurrent editing

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `editor1`, `editor2` | Both edit the same draft part and the same page at the same time | Second save refused with differences shown. No data lost. |
| 2 | `editor1`, `editor2` | Both change work item statuses on the same Board | Both see each other's changes without reloading |

### SCN-07 Approval from the phone

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `editor1` | Submit a change order with approver `appr2` | `appr2` notified |
| 2 | `appr2` on the phone | Open the notification, review Changes, approve with password | State Approved |

### SCN-08 Obsolete purchased part

| Step | User | Action | Result |
|---|---|---|---|
| 1 | `editor1` | Create a replacement COTS part and link the old part "replaced by" the new one | Link shown both ways |
| 2 | `editor1` | Check Where-used of the old part | Every assembly using it listed |
| 3 | `editor1` | One change order: revise each affected assembly to use the new part and obsolete the old part | Validation shows no errors |
| 4 | Approvers, `ra1` | Approve and release | Old part Obsolete. Assemblies at the new revision use the new part. |

### SCN-09 Backup and restore after use

| Step | User | Action | Result |
|---|---|---|---|
| 1 | Devon on the VM | Run a backup after SCN-01 to SCN-08, restore onto a fresh stack | Every scenario record, revision, baseline, comment and notification present. BOM totals unchanged. |

## 10. Prototype acceptance checks

The prototype checks from HANDOFF.md, adapted for version 1.

| Prototype check | Version 1 check |
|---|---|
| Every page returns 200; unknown item returns 404 | All checklist screens load; SEC-06 |
| Page with an embedded BOM shows the BOM table and flattened parts | PAG-09 |
| Extended quantity is correct for a multi-quantity line | ASM-03 |
| Where-used on a part lists its parent assembly | ASM-05 |
| History shows the initial commit and each later change with note and author | ITM-10 |
| Traceability flags missing verification as "No test" and complete coverage correctly | ITM-17 |
| Seed data validates with no errors | INS-06 |
| Creating an item allocates the next ID and opens the editor | NUM-01, NUM-15 |
| Validator reports a broken link | Super Admin removes a record linked from another (DATA-MODEL.md 2.7). Validation reports the broken link. Must. |
| History diff shows added and removed lines | ITM-19 |
| A stale save is rejected and the first save is kept | ITM-11 |
| Changing an item's ID in the editor is rejected | NUM-16 |
| A file without frontmatter is rejected | Devon adds a file without frontmatter to the engineering repository on the VM and restarts. Startup reports the file as invalid and continues. Should. |
| Released assembly containing a Draft part is an error | CO-06 |
| A BOM cycle is detected without hanging | ASM-10 |
| Search finds text in item bodies | SRC-03 |
| `git log`, `git status` and `git fsck --strict` succeed (the repository is bare, so `git status` is replaced by `git tag`) | CO-18 |
| A request with a foreign Host header is refused | SEC-01 |

## 11. Open items

| Item | Affects |
|---|---|
| Real items for the realistic sample | Appendix B, section 9 |

---

## Appendix A. Seed data set

Dates are relative to the load date (D). "D-3" means three days before the load date.

### A.1 Users

All seed users share the password entered when seed data is loaded.

| Username | Library role | SP-0001 role | SP-0002 role | Notes |
|---|---|---|---|---|
| `superadmin` | Super Admin | Admin | Admin | |
| `admin1` | Admin | Admin | Admin | |
| `ra1` | Release Authority | Viewer | | |
| `appr1` | Approver | Viewer | | |
| `appr2` | Approver | Viewer | | |
| `editor1` | Editor | Editor | Editor | |
| `editor2` | Editor | Editor | | |
| `contrib1` | Contributor | Contributor | | |
| `viewer1` | Viewer | Viewer | | |

### A.2 Spaces

| ID | Name | Description |
|---|---|---|
| SP-0001 | Factory Buildout | Factory build-out test space |
| SP-0002 | Product Design | Product design test space |

### A.3 Manufacturers

The initial list from DATA-MODEL.md section 7.17.

### A.4 Parts and assemblies

Home space SP-0001 unless noted. Owner `editor1`.

| ID | Name | Kind | Revision and status | Files | Notes |
|---|---|---|---|---|---|
| 100001-001 | Bracket, mounting | Production | 01 Released | Drawing | |
| 100001-002 | Bracket, mounting, left | Production | Draft, never released | | |
| 100002-001 | Back plate | Production | 01 Released, 02 In Review | Drawing | Rev 02 on CO-0002 |
| 100003-201 | DIN rail subassembly | Production | 01 Released | Drawing | |
| 100004-201 | Control panel | Production | Draft | | |
| 100005-001 | Door cover | Production | Draft | | |
| 100006-901 | Mounting interface | Production | Draft | | |
| 100007-001 | Shim | Production | 01 Released | Drawing | Locked to `admin1` and `editor1` |
| 1000001 | Keep-out zone, cable entry | Reference | Draft | | Home space SP-0002 |
| PXC.3031212 | Terminal block | COTS | 01 Released | Datasheet | Lifecycle Active |
| SIE.3RT2015-1BB41 | Contactor | COTS | 01 Released | | Lifecycle Active |
| RIT.8606000 | Enclosure | COTS | 01 Released | | Lifecycle Active |

All parts use unit of measure Each.

### A.5 Bills of Design

**100003-201 rev 01**

| Find | Child | Qty | Ref des |
|---|---|---|---|
| 10 | PXC.3031212 | 10 | |
| 20 | SIE.3RT2015-1BB41 | 2 | K1 K2 |
| 30 | 100001-001 | 2 | |

**100004-201 (Draft)**

| Find | Child | Qty | Ref des |
|---|---|---|---|
| 10 | RIT.8606000 | 1 | |
| 20 | 100002-001 | 1 | |
| 30 | 100003-201 | 3 | |
| 40 | 100001-001 | 4 | |
| 50 | 100005-001 | 1 | |
| 60 | 100007-001 | 2 | |

**Expected flattened totals for 100004-201**

| Item | Total |
|---|---|
| PXC.3031212 | 30 |
| SIE.3RT2015-1BB41 | 6 |
| 100001-001 | 10 |
| 100003-201 | 3 |
| 100007-001 | 2 |
| RIT.8606000 | 1 |
| 100002-001 | 1 |
| 100005-001 | 1 |

### A.6 Requirements, procedures and documents

| ID | Title | Status | Links | Notes |
|---|---|---|---|---|
| REQ-0001 | Control voltage | 01 Released | Satisfied by 100004-201, verified by PRC-0001 | Statement: "The control panel shall operate from a 24 VDC control supply." Priority Must, verification Test. |
| REQ-0002 | Enclosure protection | 01 Released | Satisfied by RIT.8606000 | Statement: "The enclosure shall provide IP54 protection." No verification. |
| REQ-0003 | Contactor coil voltage | Draft | Derived from REQ-0001 | No design, no verification |
| PRC-0001 | Panel functional test | 01 Released | Verifies REQ-0001, applies to 100004-201 | Type Test, 3 steps |
| DOC-0001 | Electrical design standard | 01 Released | | Type Standard, primary file PDF |
| DOC-0002 | Pricing agreement | Draft | | Type Agreement. Locked to `admin1` and `editor1`. |

### A.7 Change orders

| ID | Title | Type | State | Owner | Affected items | Approvers |
|---|---|---|---|---|---|---|
| CO-0001 | Initial release, panel components | Initial release | Released by `ra1`, baseline created | `editor1` | All items shown as rev 01 Released in A.4 and A.6 | `appr1` approved, `appr2` approved |
| CO-0002 | Back plate hole pattern | Design change | In Review | `editor1` | 100002-001 Revise 01 to 02 | `appr1` approved, `appr2` pending |
| CO-0003 | Door cover release | Initial release | Draft | `editor2` | None | None |

### A.8 Pages

| ID | Space | Title | Parent | Type | Controlled | Notes |
|---|---|---|---|---|---|---|
| PG-0001 | SP-0001 | Factory Buildout | Space home | General | No | Pinned link to PG-0002 |
| PG-0002 | SP-0001 | Control panel overview | | General | No | Bill of Design block for 100004-201. Work list block: open NCRs found on 100003-201. Comment by `editor2` mentioning `@editor1`. |
| PG-0003 | SP-0001 | Kickoff meeting notes | PG-0002 | Meeting notes | No | Two saved versions |
| PG-0004 | SP-0001 | Panel build standard | | General | Yes | 01 Released, show released only |
| PG-0005 | SP-0002 | Product Design | Space home | General | No | |
| PG-0006 | SP-0002 | Concept notes | | General | No | |

### A.9 Work items

| ID | Space | Title | Status | Assignee | Due | Other |
|---|---|---|---|---|---|---|
| EPC-0001 | SP-0001 | Control panel build | Open | `editor1` | | Children TSK-0001, TSK-0002, TSK-0003 |
| TSK-0001 | SP-0001 | Order enclosure | Open | `editor1` | D-3 | Affects RIT.8606000 |
| TSK-0002 | SP-0001 | Mount DIN rail subassemblies | Open | `editor1` | D+7 | Blocked by TSK-0001 |
| TSK-0003 | SP-0001 | Approve panel layout | Done | `editor2` | D-10 | |
| TSK-0004 | SP-0002 | Concept review | Open | `editor1` | D+14 | |
| ISS-0001 | SP-0001 | Terminal blocks too close to wire duct | Open | `editor2` | | Area Design, severity Major, affects 100003-201 |
| ACT-0001 | SP-0001 | Send meeting minutes | Open | `contrib1` | D+1 | Source PG-0003 |
| NCR-0001 | SP-0001 | Contactor coil damaged | Reported | `contrib1` | | Found on 100003-201 rev 01, quantity 2, receiving inspection, manufacturer SIE, Major, design change No |
| NCR-0002 | SP-0001 | Back plate holes misaligned | Dispositioned | `editor1` | | Found on 100002-001 rev 01, quantity 1, in process, Rework signed by `appr1`, Major, design change Yes |

### A.10 Notifications at load

| Recipient | Event | Record |
|---|---|---|
| `editor1` | Mentioned | PG-0002 |
| `appr2` | Approval requested | CO-0002 |

## Appendix B. Realistic sample

Supplied by Devon before the scenarios are run. One control panel from the factory build-out.

| Content | Quantity |
|---|---|
| Top assembly (-2XX) | 1 |
| Subassembly (-2XX), used more than once in the top assembly | 1 |
| Production piece parts (-0XX) with drawings | 2 to 4 |
| COTS parts with datasheets, from at least three manufacturers | About 15 |
| One COTS part line with a decimal quantity (for example wire in meters) | 1 |
| Requirements, at least one without a verifying procedure | 3 to 5 |
| Test procedure | 1 |
| Reference BOM with expected flattened totals, prepared outside DesignHub | 1 |
| A replacement COTS part for SCN-08 | 1 |

### B.1 Item list

| Item number | Name | Kind | Unit | Parent and quantity | File |
|---|---|---|---|---|---|
| | | | | | |
