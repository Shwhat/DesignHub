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

## Workflows

Each work item type has one workflow, used in every space. The default for tasks, issues, actions and epics is Open, In Progress, In Review, Done, with Blocked and Cancelled available from any open state.

Admins can edit a workflow in Admin, Workflows: add, rename and reorder states, choose who may make each move, and require fields before a state can be entered. Every state has a category:

| Category | Meaning |
|---|---|
| Open | Not started |
| Active | In progress |
| Done | Finished; counts toward progress |
| Cancelled | Stopped; left out of progress |

My Work, overdue checks, epic and milestone progress, sprints and reports use the categories, so they keep working when states are renamed. Each saved workflow is a new version. Items in a removed state are moved to a state the Admin chooses, and the move is recorded in each item's history. Change order states cannot be edited.

## Nonconformance workflow

The default NCR workflow:

| State | To enter this state |
|---|---|
| Reported | The required NCR fields are filled in |
| Dispositioned | A disposition is chosen and every disposition approver has signed |
| Corrective Action | |
| Verified | Root cause and corrective action are recorded |
| Closed | Verification of effectiveness is recorded. If a design change is required, a linked change order is required. |

Dispositions: Use as is, Rework, Repair, Scrap, Return to vendor.

These rules stay in force however the NCR workflow is edited:

- The Disposition state needs a disposition with every disposition approver signed. It cannot be removed.
- Done states need a root cause and a corrective action.
- Done states need a linked change order when a design change is required.

## Suppliers on NCRs

An NCR can name a supplier company. Users who can edit work items can request a supplier response with a due date; the supplier's Vendor users are notified. Vendors enter the supplier root cause and corrective action and add supplier documents. The NCR assignee and watchers are notified when the supplier responds.

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
| Sprints | Sprints of a space and the backlog of open items with no sprint |
| Milestones | Milestones with progress and health |
| Timeline | Epics, items with due dates, milestones and sprints over time, grouped by epic or milestone |
| Time report | Time logged against estimates, by person, item, epic or sprint |

## Sprints

- A sprint has a name, goal, start and end date. A space has at most one active sprint.
- Starting a sprint records the items and estimates it commits to.
- Closing a sprint moves unfinished items to the next planned sprint or back to the backlog, and records what was completed.
- The board shows the active sprint by default.

## Milestones

| Health | When |
|---|---|
| Done | Every linked item is done or cancelled |
| Late | The target date has passed |
| At risk | The target is within 7 days and less than 80 percent of items are done |
| On track | Otherwise |

The owner is notified once when a milestone becomes at risk.

## Time

- Time is logged against tasks, issues, actions and NCRs in quarter-hour steps from 0.25 to 24 hours, never for a future date. An epic totals its items.
- You can change and delete your own entries. Admins can change anyone's.
- An item shows its estimate, time logged and time remaining.
