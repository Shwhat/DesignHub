# Record fields

Field types: Text, Long text (Markdown), Number, Date, Choice (from a fixed or Admin list), User, Link, Auto (set by the system), Computed (calculated when shown).

## Common engineering item fields

Used by parts, assemblies, requirements, procedures and documents.

| Field | Type | Required | Notes |
|---|---|---|---|
| Revision | Number | Auto | Blank until first release |
| Status | Choice | Auto | See [Revisions and release](/about/revisions) |
| Owner | User | Yes | |
| Assignee | User | No | |
| Created by | User | Auto | |
| Home space | Space | Yes | |
| Labels | Text list | No | |
| Locked | User access list | No | Empty means not locked |
| Attachments | Files with role | No | |
| Links | Links | No | |

## Part

| Field | Type | Required | Notes |
|---|---|---|---|
| Item number | Text | Yes | Never changes after creation |
| Part kind | Choice | Yes | Production, Reference, Purchased |
| Name | Text, max 50 | Yes | Shown in the drawing title block |
| Description | Long text | No | Searchable |
| Unit of measure | Choice | Yes | Admin list |
| Tracking | Choice | Yes | Serial, Lot, None. Default Lot. |
| Commodity code | Choice | No | Admin list |
| Vendors may download native files | Yes/No | Yes | Default No |

Purchased parts add:

| Field | Type | Required | Notes |
|---|---|---|---|
| Manufacturer | Choice | Yes | Manufacturer list |
| Order number | Text | Yes | Raw catalog number |
| Type number | Text | No | Descriptive type designation |
| Lifecycle status | Choice | Yes | Active, Not Recommended for New Design, Obsolete |
| Approved alternates | Links to purchased parts | No | |
| Unit cost | Money | No | |
| Lead time | Days | No | |

## Bill of Design line

| Field | Type | Required | Notes |
|---|---|---|---|
| Find number | Number | Yes | Steps of 10, editable |
| Child item | Link to part or assembly | Yes | Follows the child's released revision |
| Quantity | Number | Yes | Decimals allowed |
| Unit of measure | From child item | Auto | |
| Reference designators | Text list | No | Count should match quantity |
| Line notes | Text | No | |

## Requirement

| Field | Type | Required | Notes |
|---|---|---|---|
| Title | Text, max 100 | Yes | |
| Statement | Long text | Yes | Written as a "shall" statement |
| Rationale | Long text | No | |
| Category | Choice | Yes | Admin list |
| Source | Text | No | |
| Verification method | Choice, multiple | Yes | Inspection, Analysis, Demonstration, Test |
| Priority | Choice | Yes | Must, Should, Could |
| Coverage | Computed | Auto | Satisfied by and verified by, with gaps shown |

## Procedure

| Field | Type | Required | Notes |
|---|---|---|---|
| Title | Text, max 100 | Yes | |
| Procedure type | Choice | Yes | Admin list |
| Purpose and scope | Long text | Yes | |
| Applies to | Links to parts or assemblies | No | |
| Tools and equipment | Text list | No | |
| Safety and prerequisites | Long text | No | |
| Estimated duration | Minutes | No | |
| Steps | Instruction and acceptance criteria per step | Yes | Renumber when moved |

## Document

| Field | Type | Required | Notes |
|---|---|---|---|
| Title | Text, max 100 | Yes | |
| Document type | Choice | Yes | Admin list |
| Primary file | Attachment | Yes, at release | |
| Summary | Long text | No | Searchable |
| Applies to | Links to engineering items | No | |
| External reference | Text | No | For example a standard number |

## Page

| Field | Type | Required | Notes |
|---|---|---|---|
| Title | Text, max 150 | Yes | |
| Space and parent page | | Yes | Empty parent means top level |
| Page type | Choice | Yes | Admin list |
| Controlled | Choice | Yes | Use type default, Yes, No |
| Show released only | Yes/No | No | Controlled pages only |
| Body | Long text | No | May contain live blocks |
| Owner | User | Yes | |
| Template | Template | No | Set when the page is created from a template |

## Change order

| Field | Type | Required | Notes |
|---|---|---|---|
| Title | Text, max 100 | Yes | |
| Change type | Choice | Yes | Initial release, Design change, Documentation only, Obsolescence |
| Reason for change | Long text | Yes | |
| Description of change | Long text | Yes | |
| Affected items | Item and action (Release, Revise, Obsolete) | Yes | |
| Existing stock disposition | Choice | No | Use as is, Rework, Scrap, Return to vendor |
| Need date | Date | No | |
| Priority | Choice | Yes | Low, Normal, High, Urgent |
| Approvers | Users | Yes, before review | Decision, time, comment and signature per approver |
| Released by, Released on, Baseline | | Auto | Set at release |

## Work items

| Field | Type | Required | Notes |
|---|---|---|---|
| Title | Text, max 150 | Yes | |
| Description | Long text | No | |
| Priority | Choice | Yes | Low, Normal, High, Urgent |
| Assignee | User | No | |
| Reporter | User | Auto | Defaults to creator, editable |
| Due date | Date | No | |
| Epic | Link to epic | No | |
| Issue area and severity | Choice | Issues | Severity: Minor, Major, Critical |
| Epic start, target and progress | | Epics | Progress is the share of child items in a Done state |
| Estimate | Hours, in quarter-hour steps | No | |
| Time logged, Remaining | Hours | Computed | Remaining is estimate minus time logged, not below zero |
| Sprint | Sprint in the same space | No | Not on epics |
| Milestone | Milestone in the same space | No | |

## Nonconformance (NCR)

| Field | Type | Required | Notes |
|---|---|---|---|
| Found on | Part or assembly, pinned to revision | Yes | |
| Serial or lot number | Text | No | |
| Quantity affected | Number | Yes | |
| Where found | Choice | Yes | Receiving inspection, In process, Final test, Field |
| Manufacturer | Choice | No | |
| PO or work order number | Text | No | |
| Description | Long text | Yes | |
| Requirement not met | Text or link | No | |
| Severity | Choice | Yes | Minor, Major, Critical |
| Containment | Long text | No | |
| Disposition and approvers | | To reach Dispositioned | Signed by each approver |
| Root cause, corrective action | Long text | To reach Verified | |
| Verification of effectiveness | Long text | To reach Closed | |
| Requires design change | Yes/No | Yes | If Yes, a linked change order is required to close |
| Supplier | Company | No | Gives that company's Vendors access to the NCR |
| Supplier response requested, due | Date | No | |
| Supplier root cause, corrective action | Long text | No | Entered by the supplier's Vendors |
| Supplier responded on | Date and time | Auto | |

With an edited NCR workflow, the state requirements above follow the workflow; the fixed rules on the [Work and NCRs](/about/work) page always apply.

## Company

| Field | Type | Required | Notes |
|---|---|---|---|
| ID | COMP-NNNN | Auto | |
| Name | Text, max 100 | Yes | Unique |
| Kind | Choice list | Yes | Vendor, Contractor, Customer, Other |
| Supplier status | Choice | No | Approved, Conditional, Not approved. A warning shows on parts and NCRs when not Approved. |
| Manufacturer codes | Manufacturer list | No | |
| Address, phone, website | Text | No | |
| Contacts | Name, role, email, phone | No | Contacts are not users |
| Notes | Long text | No | |
| Active | Yes/No | Yes | Inactive companies cannot be linked, and their Vendor and Contractor accounts cannot sign in |

## Sprint, milestone and time entry

| Record | Fields |
|---|---|
| Sprint (SPR) | Name, space, goal, start and end date, state (Planned, Active, Closed), committed and completed items |
| Milestone (MS) | Name, space, target date, description, owner, progress, health, completed on |
| Time entry | Work item, person, date, hours (0.25 to 24), note |

## Template

| Field | Type | Required | Notes |
|---|---|---|---|
| ID | TPL-NNNN | Auto | |
| Name | Text, max 60 | Yes | |
| Scope | All spaces or one space | Yes | |
| Description | Text | No | Shown in the New page dialog |
| Page type | Choice | Yes | |
| Title pattern | Text | No | For example "Design review {date}" |
| Body | Long text | Yes | Placeholders {date}, {user}, {space} and {title} are filled when the page is created |
| Active | Yes/No | Yes | |

## Link types

| Link | From | To | Shown on the target as |
|---|---|---|---|
| contains | Assembly | Part or assembly | used in |
| satisfies | Part, assembly | Requirement | satisfied by |
| verifies | Procedure | Requirement | verified by |
| derived from | Requirement | Requirement | derives |
| affects | Work item, change order | Engineering item | affected by |
| found on | Issue, NCR | Part or assembly | has finding |
| fixed by | Issue, NCR | Change order | fixes |
| blocks | Work item | Work item | blocked by |
| duplicates | Work item | Work item | duplicated by |
| supplied by | Part, assembly | Company | supplies |
| supplier | NCR | Company | has nonconformance |
| replaced by | Any record | Record of the same type | replaces |
| relates to | Any record | Any record | relates to |
