# Numbering

## Principles

- Every record has a permanent identifier. It never changes and is never reused.
- An identifier is unique across all records of all types.
- Significance in a number is kept to the minimum needed for design, assembly and configuration management.
- The revision is not part of the identifier. It is shown beside it, for example `100042-001 rev 02`.

## Production parts and assemblies

Format: 6-digit base number, dash, 3-digit suffix. Example: `100042-001`.

- The base number carries no meaning. DesignHub offers the next free base number from a single counter, with suffix `-001`.
- The suffix is partly meaningful. Its first digit gives the category. The second and third digits are sequential and used as the creator sees fit.
- The user may change the base number and suffix. The full number must not already be used or retired.
- The number is reserved when the item is created. The counter skips any base number already used in any form.

| Suffix | Category | Has a Bill of Design | Notes |
|---|---|---|---|
| -000 | Layout | No | Parts modeled from one layout share its base number |
| -0XX | Piece part | No | Single part with a drawing or specification |
| -1XX | Piece part (spillover) | No | Same rules as -0XX |
| -2XX | Assembly | Yes | Two or more parts; the drawing includes the Bill of Design |
| -3XX | Harness | Yes | |
| -4XX | PCB | No | Includes flexible printed circuits |
| -5XX | PCBA | Yes | |
| -6XX | Internally developed electronic component | No | |
| -7XX, -8XX | Reserved | | |
| -9XX | Interface or make-on-assembly part | No | Cannot be added as a Bill of Design line |

## Reference parts

Format: 7-digit number from its own counter. Example: `1000042`.

- Used for concept models, clearance checks, keep-out zones and other reference-only purposes. Never ordered or built.
- A reference part can be converted to a production part. Conversion issues a new production number and adds a `replaced by` link from the reference part.
- Because the base numbers have different lengths, production and reference numbers never collide.

## Purchased parts

Format: manufacturer code, period, manufacturer order or catalog number.

| Example | Meaning |
|---|---|
| `PXC.3031212` | Phoenix Contact catalog number 3031212 |
| `SIE.3RT2015-1BB41` | Siemens order number 3RT2015-1BB41 |
| `RIT.8606000` | Rittal catalog number 8606000 |

- The manufacturer code comes from the manufacturer list. A code cannot change once it is used in a part number, and inactive codes cannot be used for new parts.
- The raw catalog number is also stored in the Order number field, and the descriptive designation in the Type number field.
- Two manufacturers that share a catalog number never collide.

## All other records

Format: prefix, dash, number, with one counter per type. Numbers grow past 9999 when needed.

| Record | Prefix |
|---|---|
| Requirement | REQ |
| Procedure | PRC |
| Document | DOC |
| Page | PG |
| Change order | CO |
| Space | SP |
| Task | TSK |
| Issue | ISS |
| Nonconformance | NCR |
| Action | ACT |
| Epic | EPC |

## Retirement

- Released records are never deleted. They are made Obsolete or Cancelled.
- A draft that has never been released can be deleted. Its number is retired and never issued again.
- A record's type never changes. A record created with the wrong type is retired and replaced, with a `replaced by` link.
