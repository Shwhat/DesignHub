# Work and NCRs

## Work item types

| Type | Prefix | Purpose |
|---|---|---|
| Task | TSK | A unit of work with an owner and due date |
| Issue | ISS | A problem in the design, the factory or a process |
| Nonconformance | NCR | A part or build that does not meet specification |
| Action | ACT | A follow-up from a review, meeting or audit |
| Epic | EPC | A group of related work items |

Every work item belongs to one space.

## Default workflow

Tasks, issues, actions and epics: Open, In Progress, In Review, Done. Blocked and Cancelled are available from any open state.

## Nonconformance workflow

| State | To enter this state |
|---|---|
| Reported | The required NCR fields are filled in |
| Dispositioned | A disposition is chosen and every disposition approver has signed |
| Corrective Action | |
| Verified | Root cause and corrective action are recorded |
| Closed | Verification of effectiveness is recorded. If a design change is required, a linked change order is required. |

Dispositions: Use as is, Rework, Repair, Scrap, Return to vendor.

An NCR records where it was found (receiving inspection, in process, final test or field), the part or assembly and revision it was found on, the quantity affected, severity, containment, and the serial or lot number.

## Links between work and other records

| Link | Meaning |
|---|---|
| affects | The work item or change order affects an engineering item |
| found on | The issue or NCR was found on a part or assembly |
| fixed by | The issue or NCR is fixed by a change order |
| blocks / blocked by | One work item must finish before another |
| duplicates | The work item repeats another |
| relates to | Any general relation |

## Views

| View | Shows |
|---|---|
| Board | Columns by status, with drag and drop |
| List | Sortable, filterable table with saved filters and column choice |
| My Work | Items assigned to you, overdue first |
