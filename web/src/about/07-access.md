# Access and sign-in

## Library and spaces

| Area | Contains | Access controlled by |
|---|---|---|
| Library | Parts, assemblies, requirements, procedures, documents, change orders, companies | Library role (company-wide) |
| Spaces | Pages, tasks, issues, NCRs, actions, epics | Space role (per space) |

- Each user holds one library role and may hold one role in each space.
- Every library record has a home space, used for filtering and ownership. It does not limit access.
- People fields on a record, such as owner and assignee, record who is responsible. They give no extra permissions.

## Roles

| Role | Can |
|---|---|
| Super Admin | Everything, including system settings, sign-in configuration and backups. Manages Admins. |
| Admin | Manage users and roles (except Super Admins), spaces, lists and settings |
| Release Authority | Release approved change orders and create baselines |
| Approver | Approve or reject change orders and NCR dispositions assigned to them |
| Editor | Create and edit engineering items, pages and work items. Open change orders. |
| Contributor | Create and update work items and NCRs, log time. Read-only on engineering data. |
| Contractor | Edit pages and work items in the spaces they belong to. Read released library items whose home space is one of those spaces. Nothing else. |
| Vendor | See released revisions of items their company supplies, NCRs naming their company, and actions assigned to them. Respond to supplier requests and add supplier documents. |
| Viewer | Read only |

At least one active Super Admin always exists.

Admins with Manage roles and workflows can copy any role except Super Admin, rename the copy and change its permissions. Nobody can grant a permission they do not have. A custom role in use can be deactivated but not deleted.

## Vendors and Contractors

- Every Vendor and Contractor account is linked to one company and has an expiry date.
- Vendors see only released revisions, and only the files in the roles Drawing, Datasheet, 3D model, Schematic and BOM file. Native design files are shown when the item allows Vendor download.
- Vendors never see spaces, pages, requirements, procedures, documents, change orders or other companies.
- Comments marked Internal are hidden from Vendors.
- On its expiry date an account can no longer sign in. Admins are notified 14 days and 1 day before. Deactivating a company also stops its accounts.

## Record locking

- Any record can be locked to a list of people.
- Anyone not on the list does not see the record at all: not in search, lists, links, Bills of Design or notifications.
- Bill of Design quantities and validation still include locked items, so totals stay correct.

## Sign-in

| Method | Rules |
|---|---|
| Password | At least 10 characters and not the username. Stored as an Argon2id hash. |
| Google | Available when a Super Admin has configured it. Only Google Workspace accounts of an allowed domain, or individually allowed addresses, can sign in. The address must be verified by Google. |

- A new Google user is created as a Viewer with no space access. An Admin then assigns roles.
- An existing account is linked to Google on its first Google sign-in when exactly one account has that email address. After linking, that account signs in with Google only.
- A Super Admin who signs in with Google can turn off password sign-in for everyone.
- If no administrator can sign in, the server's recovery command allows one password sign-in for a Super Admin within one hour.
- Two-step verification is provided through Google sign-in.

## Sessions

- A session ends after 30 minutes without activity.
- "Keep me signed in" keeps a session for 1 day.
- Five failed sign-in attempts block further attempts for that username or address for 15 minutes.
- You can see your active sessions and sign out of other devices in Settings, Security.
- A changed password signs out every other session.

## Protection

- Session cookies are HTTP-only, and secure when served over HTTPS.
- Every change request is checked against cross-site request forgery.
- An allowed-addresses setting limits the host names and IP addresses DesignHub answers to.
- The browser is told not to load content from other sites and not to show DesignHub inside other sites.
- Notifications are never sent about records the recipient cannot see.
