# Revisions and release

## Revision scheme

- Revisions are numeric: 01, 02, 03.
- The first released revision is 01.
- A new record has no revision until it is first released.

## Lifecycle

**Development.** A new item is created with its permanent number and status Draft. Edits are unrestricted, and every save is recorded with user, time and comment. No change order is needed.

**First release.** An item is released when something physical depends on it: it is ordered, built, tested, or handed to a vendor or the factory. Release happens through a change order and makes the item revision 01. Related items are released together on the same change order.

**After release.** A released revision is locked. When the owner is ready to change the item, they start the next revision (for example 02) as a draft. Starting a revision does not need a change order. The new revision becomes released only when a change order releases it. Until then, the previous revision stays the released revision.

## Statuses

Status belongs to each revision.

| Status | Meaning | Editable |
|---|---|---|
| Draft | Working revision, not yet released | Yes |
| In Review | Included in a change order under review | Only through the change order |
| Released | The current approved revision | No |
| Superseded | An earlier released revision replaced by a newer one | No |
| Obsolete | The item is retired. No new builds or orders. | No |

Example: 01 Released, 02 Draft, 02 In Review, then 02 Released and 01 Superseded.

An item becomes Obsolete only through a change order.

## Links and revisions

- A link points to the record in general and follows its current released revision.
- A link can be pinned to a specific revision, for example an NCR found on `100042-201 rev 02`.

## Attachments

- Every attachment belongs to a specific revision. A released revision's files never change.
- Files are not edited in place. Uploading a new version replaces the file in the draft revision only.
- A file attached to several revisions is stored once.
- Every attachment has a file role. A missing expected role produces a warning at release. It does not block release.

| Item | Expected files |
|---|---|
| -000 Layout | 3D model |
| -0XX, -1XX Piece part | Drawing |
| -2XX Assembly | Drawing |
| -3XX Harness | Drawing, Schematic, 3D model |
| -4XX PCB | Schematic, Drawing, BOM file |
| -5XX PCBA | Schematic, Drawing, BOM file |
| -6XX Internal electronic component | Schematic, Drawing |
| -9XX Interface or make-on-assembly | None |
| Purchased parts | Datasheet |

## Bill of Design rules

- The Bill of Design is part of the assembly revision. Changing the Bill of Design of a released assembly needs a new revision.
- Find numbers are assigned in steps of 10 and can be edited.
- A line follows the child item's current released revision.
- Quantities may be decimal. The unit of measure comes from the child item.
- The number of reference designators should match the quantity. A mismatch is a warning.
- -9XX parts cannot be added as lines.

## Validation

| Check | Result |
|---|---|
| A line points to an item that does not exist | Error |
| An assembly contains itself at any level | Error |
| A released assembly contains an unreleased item | Error |
| Reference designator count differs from quantity | Warning |
| An expected file role is missing | Warning |

Errors block approval of a change order. Warnings do not.
