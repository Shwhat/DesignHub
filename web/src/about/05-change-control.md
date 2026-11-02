# Change control

## What needs a change order

| Change | Change order needed |
|---|---|
| Editing a draft item | No |
| Starting a new draft revision of a released item | No |
| Releasing an item for the first time | Yes |
| Releasing a new revision | Yes |
| Making an item Obsolete | Yes |
| Editing a controlled page | Yes |
| Changing a page from controlled to uncontrolled | Yes |

## Change order contents

A change order records the reason for the change, a description, the affected items with the action for each (Release, Revise or Obsolete), the approvers, priority, need date and the disposition of existing stock.

Each change order works on its own branch of the engineering repository, so its edits stay separate from released data until release.

## States

Draft, In Review, Approved, Released, Rejected, Cancelled.

## Review and approval

- The review view shows every change the change order would make, compared with the current released data.
- Approvers are assigned per change order. Only roles allowed to manage approvers can add or remove them.
- The author cannot be the only approver.
- Every assigned approver must approve. A single rejection rejects the change order.
- A change order cannot be approved while validation errors exist on its affected items.
- When a change order is rejected or cancelled, its affected items keep their current status. An Admin setting can instead return them to Draft.

## Electronic signatures

- Each approval, rejection, release and NCR disposition decision is an electronic signature.
- The signer confirms with their password, or with their signing PIN if they sign in with Google.
- Each signature records the signer, decision, date and time, and comment.
- Failed signature attempts are recorded in the audit log and count toward the sign-in attempt limit.

## Release

- A Release Authority releases an approved change order, with an electronic signature.
- Release merges the changes, makes each affected draft revision the released revision, marks the previous revision Superseded, and creates a signed baseline.
- Any baseline can be viewed exactly as released and compared with any other baseline.

## Controlled pages

- Every page type has a Controlled setting. Admins set the default per type, and a page can override it.
- A controlled page changes only through a change order.
- A controlled page can show only its released revision, so readers always see the approved version.
