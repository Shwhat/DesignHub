<script lang="ts">
  import { get, post, patch, put } from '../../lib/api';
  import { session, can, toast, loadUsers, refreshMe } from '../../lib/state.svelte';
  import { navigate } from '../../lib/router.svelte';
  import { dateTime, bytes } from '../../lib/format';
  import { loadLists } from '../../lib/lists.svelte';
  import Modal from '../../components/Modal.svelte';
  import Sortable from 'sortablejs';
  import { tick, onDestroy, untrack } from 'svelte';
  let { params }: { params: any; pattern?: string } = $props();
  const userId = untrack(() => params.id);
  const section = untrack(() => params.section) || (can('users.manage') ? 'users' : can('system.manage') ? 'system' : 'lists');
  const menu = [
    ['users', 'Users', 'users.manage'], ['roles', 'Roles', 'users.manage'], ['spaces', 'Spaces', 'spaces.manage'], ['manufacturers', 'Manufacturers', 'lists.manage'],
    ['lists', 'Lists', 'lists.manage'], ['signin', 'Sign-in message', 'users.manage'], ['audit', 'Audit log', 'audit.view'], ['system', 'System', 'system.manage'],
  ].filter(([, , p]) => can(p));

  // Users ----------------------------------------------------------------
  let users: any[] = $state([]);
  let roles: any[] = $state([]);
  let perms: any[] = $state([]);
  let spaces: any[] = $state([]);
  let newUser: any = $state(null);
  let user: any = $state(null);
  let resetPw = $state('');
  async function loadUsersAdmin() { users = await get('/api/users'); }
  async function loadRoles() { const r = await get('/api/roles'); roles = r.roles; perms = r.permissions; }
  async function loadSpaces() { spaces = await get('/api/spaces'); }
  if (section === 'users') { loadUsersAdmin(); loadRoles(); loadSpaces(); if (userId) loadUser(userId); }
  async function loadUser(id: string) { user = await get('/api/users/' + id); }
  async function createUser() {
    try { await post('/api/users', newUser); newUser = null; toast('User created.'); loadUsersAdmin(); loadUsers(); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function updateUser(body: any, msg = 'Saved.') {
    try { await patch('/api/users/' + user.id, body); toast(msg); loadUser(user.id); loadUsersAdmin(); loadUsers(); } catch (e: any) { toast(e.message, 'error'); }
  }

  // Roles ----------------------------------------------------------------
  if (section === 'roles') loadRoles();
  async function toggleApprovers(r: any) {
    try { await patch('/api/roles/' + r.id, { canManageApprovers: !r.canManageApprovers }); loadRoles(); refreshMe(); } catch (e: any) { toast(e.message, 'error'); }
  }

  // Spaces ---------------------------------------------------------------
  let newSpace: any = $state(null);
  let space: any = $state(null);
  let members: any[] = $state([]);
  let memberUser = $state('');
  let memberRole = $state('Viewer');
  let pageTypes: string[] = $state([]);
  if (section === 'spaces') { loadSpaces(); loadRoles(); get('/api/lists').then((ls: any[]) => (pageTypes = ls.find((l) => l.key === 'page_types').values.map((v: any) => v.value))); }
  async function openSpace(s: any) { space = { ...s }; members = await get(`/api/spaces/${s.id}/members`); }
  async function createSpace() {
    try { const r = await post('/api/spaces', newSpace); newSpace = null; toast(`${r.id} created.`); loadSpaces(); refreshMe(); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function saveSpace(body: any) {
    try { await patch('/api/spaces/' + space.id, body); toast('Saved.'); loadSpaces(); refreshMe(); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function setMember(userId: string, role: string) {
    try { await put(`/api/spaces/${space.id}/members`, { userId, role }); members = await get(`/api/spaces/${space.id}/members`); refreshMe(); } catch (e: any) { toast(e.message, 'error'); }
  }

  // Manufacturers ----------------------------------------------------------
  let mfrs: any[] = $state([]);
  let newMfr = $state({ code: '', name: '', website: '' });
  async function loadMfrs() { mfrs = await get('/api/manufacturers'); }
  if (section === 'manufacturers') loadMfrs();
  async function addMfr() {
    try { await post('/api/manufacturers', newMfr); newMfr = { code: '', name: '', website: '' }; loadMfrs(); loadLists(true); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function updMfr(m: any, body: any) {
    try { await patch('/api/manufacturers/' + m.code, body); loadMfrs(); loadLists(true); } catch (e: any) { toast(e.message, 'error'); loadMfrs(); }
  }

  // Lists ------------------------------------------------------------------
  let lists: any[] = $state([]);
  let listKey = $state('units');
  let values: any[] = $state([]);
  let listBody: HTMLElement | undefined = $state();
  let sortable: Sortable | null = null;
  const expectKeys = ['000', '0XX', '1XX', '2XX', '3XX', '4XX', '5XX', '6XX', '9XX', 'COTS'];
  if (section === 'lists') get('/api/lists').then((l) => { lists = l; pickList(listKey); });
  async function pickList(k: string) {
    listKey = k;
    values = JSON.parse(JSON.stringify(lists.find((l) => l.key === k)?.values || []));
    await tick();
    sortable?.destroy();
    if (listBody) sortable = Sortable.create(listBody, { handle: '.drag', animation: 120, onEnd: (e) => { const v = [...values]; const [m] = v.splice(e.oldIndex!, 1); v.splice(e.newIndex!, 0, m); values = v; } });
  }
  onDestroy(() => sortable?.destroy());
  async function saveList() {
    try { const r = await put('/api/lists/' + listKey, values); lists = lists.map((l) => (l.key === listKey ? { ...l, values: r } : l)); values = r; toast('List saved.'); loadLists(true); } catch (e: any) { toast(e.message, 'error'); }
  }

  // Sign-in message, system ------------------------------------------------
  let sys: any = $state(null);
  let status: any = $state(null);
  let hostsText = $state('');
  if (section === 'signin' || section === 'system') get('/api/admin/system').then((s) => { sys = s; hostsText = (s.allowedHosts || []).join('\n'); });
  if (section === 'system') get('/api/admin/status').then((s) => (status = s));
  // Google sign-in ---------------------------------------------------------
  let gs: any = $state(null);
  let gDomains = $state('');
  let gEmails = $state('');
  let gSecret = $state('');
  function showGoogle(v: any) { gs = v; gDomains = v.allowedDomains.join('\n'); gEmails = v.allowedEmails.join('\n'); gSecret = ''; }
  if (section === 'system') get('/api/admin/google').then(showGoogle).catch(() => {});
  async function saveGoogle(body: any, done = 'Saved.', box?: HTMLInputElement) {
    try { showGoogle(await put('/api/admin/google', body)); toast(done); } catch (e: any) { if (box) box.checked = !box.checked; toast(e.message, 'error'); }
  }
  function googleFields() {
    return { clientId: gs.clientId, clientSecret: gSecret, publicUrl: gs.publicUrl, allowedDomains: gDomains.split(/[\s,]+/).filter(Boolean), allowedEmails: gEmails.split(/[\s,]+/).filter(Boolean) };
  }
  async function saveSys(body: any) {
    try { sys = await put('/api/admin/system', body); hostsText = (sys.allowedHosts || []).join('\n'); toast('Saved.'); } catch (e: any) { toast(e.message, 'error'); }
  }
  let backingUp = $state(false);
  async function backupNow() {
    backingUp = true;
    try { const r = await post('/api/admin/backup'); toast(`Backup ${r.name} written.`); status = await get('/api/admin/status'); } catch (e: any) { toast(e.message, 'error'); }
    backingUp = false;
  }

  // Audit --------------------------------------------------------------------
  let audit: any[] = $state([]);
  let auditQ = $state('');
  async function loadAudit(more = false) {
    const before = more && audit.length ? audit[audit.length - 1].id : '';
    const r = await get(`/api/admin/audit?q=${encodeURIComponent(auditQ)}${before ? '&before=' + before : ''}`);
    audit = more ? [...audit, ...r] : r;
  }
  if (section === 'audit') loadAudit();
</script>

<div class="content">
  <h1>Admin</h1>
  <div class="tabs">{#each menu as [k, l]}<button class:active={section === k} onclick={() => navigate('/admin/' + k)}>{l}</button>{/each}</div>

  {#if section === 'users'}
    {#if params.id && user}
      <a href="/admin/users">All users</a>
      <div class="card card-body" style="margin-top: 0.75rem">
        <h2>{user.displayName} <span class="muted small">{user.username}</span></h2>
        <div class="grid2">
          <div class="field"><label for="ud">Display name</label><input id="ud" bind:value={user.displayName} /></div>
          <div class="field"><label for="ue">Email</label><input id="ue" bind:value={user.email} /></div>
          <div class="field"><label for="ur">Library role</label><select id="ur" bind:value={user.libraryRole}>{#each roles as r}{#if r.name !== 'Super Admin' || session.me?.libraryRole === 'Super Admin'}<option>{r.name}</option>{/if}{/each}</select></div>
        </div>
        <div class="row end"><button class="primary" onclick={() => updateUser({ displayName: user.displayName, email: user.email, libraryRole: user.libraryRole })}>Save</button></div>
        <h3>Space roles</h3>
        <table class="data"><tbody>{#each spaces as s}<tr><td>{s.name}</td><td><select style="width: auto" value={user.spaceRoles[s.id] || ''} onchange={(e) => updateUser({ spaceRoles: { [s.id]: (e.target as HTMLSelectElement).value } })}><option value="">No access</option>{#each roles as r}<option>{r.name}</option>{/each}</select></td></tr>{/each}</tbody></table>
        {#if user.signInMethod === 'google'}
          <h3 style="margin-top: 1.25rem">Sign-in</h3>
          <p class="small muted">Signs in with Google as {user.email}.</p>
        {:else}
        <h3 style="margin-top: 1.25rem">Password reset</h3>
        <div class="row"><input type="password" style="max-width: 260px" placeholder="New password" autocomplete="new-password" bind:value={resetPw} /><button disabled={resetPw.length < 10} onclick={() => { updateUser({ password: resetPw }, 'Password reset. The user was signed out.'); resetPw = ''; }}>Reset password</button></div>
        {/if}
        <h3 style="margin-top: 1.25rem">Account</h3>
        {#if user.active}<button class="danger" onclick={() => confirm(`Disable ${user.username}? They can no longer sign in.`) && updateUser({ active: false }, 'User disabled.')}>Disable user</button>
        {:else}<button onclick={() => updateUser({ active: true }, 'User enabled.')}>Enable user</button>{/if}
        <p class="small muted">Users are never deleted, so their history stays attributed.</p>
      </div>
    {:else}
      <div class="row end" style="margin-bottom: 0.75rem"><button class="primary" onclick={() => (newUser = { username: '', displayName: '', email: '', password: '', libraryRole: 'Viewer', spaceRoles: {} })}>New user</button></div>
      <div class="card table-wrap"><table class="data">
        <thead><tr><th>Username</th><th>Display name</th><th class="hide-phone">Email</th><th>Library role</th><th>Active</th><th class="hide-phone">Sign-in</th><th class="hide-phone">Last sign-in</th></tr></thead>
        <tbody>{#each users as u}<tr class="clickable" onclick={() => navigate('/admin/users/' + u.id)}><td>{u.username}</td><td>{u.displayName}</td><td class="hide-phone">{u.email}</td><td>{u.libraryRole}</td><td>{u.active ? 'Yes' : 'No'}</td><td class="hide-phone">{u.signInMethod === 'google' ? 'Google' : 'Password'}</td><td class="hide-phone">{dateTime(u.lastSignIn)}</td></tr>{/each}</tbody>
      </table></div>
    {/if}
  {:else if section === 'roles'}
    <p class="small muted">Built-in roles. Each user has one library role and one role per space.</p>
    <div class="card table-wrap"><table class="data">
      <thead><tr><th>Permission</th>{#each roles as r}<th>{r.name}</th>{/each}</tr></thead>
      <tbody>
        {#each perms as p}<tr><td>{p.Label}</td>{#each roles as r}<td style="text-align: center">{r.permissions.includes(p.Key) ? '✓' : ''}</td>{/each}</tr>{/each}
        <tr><td>Add or remove change order approvers</td>{#each roles as r}<td style="text-align: center"><input type="checkbox" checked={r.canManageApprovers} onchange={() => toggleApprovers(r)} aria-label="{r.name} manages approvers" /></td>{/each}</tr>
      </tbody>
    </table></div>
  {:else if section === 'spaces'}
    <div class="row end" style="margin-bottom: 0.75rem"><button class="primary" onclick={() => (newSpace = { name: '', description: '' })}>New space</button></div>
    <div class="card table-wrap"><table class="data"><thead><tr><th>ID</th><th>Name</th><th>Archived</th></tr></thead>
      <tbody>{#each spaces as s}<tr class="clickable" onclick={() => openSpace(s)}><td class="id">{s.id}</td><td>{s.name}</td><td>{s.archived ? 'Yes' : 'No'}</td></tr>{/each}</tbody></table></div>
    {#if space}
      <Modal title="{space.id} {space.name}" wide onclose={() => (space = null)}>
        <div class="grid2">
          <div class="field"><label for="sn">Name</label><input id="sn" bind:value={space.name} /></div>
          <div class="field"><label for="sd">Description</label><input id="sd" bind:value={space.description} /></div>
        </div>
        <div class="row"><button class="primary" onclick={() => saveSpace({ name: space.name, description: space.description })}>Save</button>
          <button onclick={() => { saveSpace({ archived: !space.archived }); space.archived = !space.archived; }}>{space.archived ? 'Unarchive' : 'Archive'}</button></div>
        <h3 style="margin-top: 1rem">Controlled pages by default</h3>
        <div class="row">{#each pageTypes as t}<label class="checkbox"><input type="checkbox" checked={!!space.controlledDefaults[t]} onchange={(e) => { space.controlledDefaults = { ...space.controlledDefaults, [t]: (e.target as HTMLInputElement).checked }; saveSpace({ controlledDefaults: space.controlledDefaults }); }} /> {t}</label>{/each}</div>
        <h3 style="margin-top: 1rem">Members</h3>
        <table class="data"><tbody>{#each members as m}<tr><td>{m.name}</td><td><select style="width: auto" value={m.role} onchange={(e) => setMember(m.userId, (e.target as HTMLSelectElement).value)}>{#each roles as r}<option>{r.name}</option>{/each}</select></td><td><button class="small danger" onclick={() => setMember(m.userId, '')}>Remove</button></td></tr>{/each}</tbody></table>
        <div class="row" style="margin-top: 0.5rem">
          <select style="width: auto" bind:value={memberUser}><option value="">Add a person</option>{#each session.users.filter((u) => u.active && !members.some((m) => m.userId === u.id)) as u}<option value={u.id}>{u.displayName}</option>{/each}</select>
          <select style="width: auto" bind:value={memberRole}>{#each roles as r}<option>{r.name}</option>{/each}</select>
          <button disabled={!memberUser} onclick={() => { setMember(memberUser, memberRole); memberUser = ''; }}>Add</button>
        </div>
      </Modal>
    {/if}
  {:else if section === 'manufacturers'}
    <div class="card table-wrap"><table class="data">
      <thead><tr><th>Code</th><th>Name</th><th class="hide-phone">Website</th><th>Active</th><th>Used in part numbers</th></tr></thead>
      <tbody>
        {#each mfrs as m (m.code)}<tr><td class="id">{m.code}</td><td><input value={m.name} onchange={(e) => updMfr(m, { name: (e.target as HTMLInputElement).value })} aria-label="Name" /></td>
          <td class="hide-phone"><input value={m.website} onchange={(e) => updMfr(m, { website: (e.target as HTMLInputElement).value })} aria-label="Website" /></td>
          <td><input type="checkbox" checked={m.active} onchange={() => updMfr(m, { active: !m.active })} aria-label="Active" /></td><td>{m.used ? 'Yes' : 'No'}</td></tr>{/each}
        <tr><td><input style="width: 8ch" maxlength="5" placeholder="Code" bind:value={newMfr.code} /></td><td><input placeholder="Name" bind:value={newMfr.name} /></td><td class="hide-phone"><input placeholder="Website" bind:value={newMfr.website} /></td><td colspan="2"><button class="primary small" onclick={addMfr}>Add</button></td></tr>
      </tbody>
    </table></div>
    <p class="small muted">Codes are 2 to 5 capital letters. A code used in a part number cannot change. Inactive codes cannot be used for new parts.</p>
  {:else if section === 'lists'}
    <div class="row" style="margin-bottom: 0.75rem"><label for="lk" style="margin: 0">List</label><select id="lk" style="width: auto" value={listKey} onchange={(e) => pickList((e.target as HTMLSelectElement).value)}>{#each lists as l}<option value={l.key}>{l.name}</option>{/each}</select></div>
    <div class="card table-wrap"><table class="data">
      <thead><tr><th></th><th>Value</th><th>Active</th>{#if listKey === 'file_roles'}<th>Expected on</th>{/if}</tr></thead>
      <tbody bind:this={listBody}>
        {#each values as v, i (v.value + i)}<tr><td class="drag" style="cursor: grab">⠿</td><td><input bind:value={v.value} aria-label="Value" /></td><td><input type="checkbox" bind:checked={v.active} aria-label="Active" /></td>
          {#if listKey === 'file_roles'}<td>{#each expectKeys as k}<label class="checkbox" style="display: inline-flex; margin-right: 0.5rem"><input type="checkbox" checked={(v.expectedOn || []).includes(k)} onchange={(e) => { const on = (e.target as HTMLInputElement).checked; v.expectedOn = on ? [...(v.expectedOn || []), k] : (v.expectedOn || []).filter((x: string) => x !== k); }} /> {k}</label>{/each}</td>{/if}</tr>{/each}
      </tbody>
    </table></div>
    <div class="row" style="margin-top: 0.75rem"><button onclick={() => (values = [...values, { value: '', active: true, expectedOn: [] }])}>Add value</button><span class="spacer"></span><button class="primary" onclick={saveList}>Save list</button></div>
    <p class="small muted">Drag to reorder. Values in use are deactivated, never deleted.</p>
  {:else if section === 'signin'}
    {#if sys}<div class="card card-body">
      <div class="field"><label for="sm">Message shown above the sign-in form (empty hides it)</label><textarea id="sm" bind:value={sys.signInMessage}></textarea></div>
      <div class="row end"><button class="primary" onclick={() => saveSys({ signInMessage: sys.signInMessage })}>Save</button></div>
    </div>{/if}
  {:else if section === 'audit'}
    <div class="row" style="margin-bottom: 0.75rem"><input style="max-width: 320px" placeholder="Filter by action, user, record or detail" bind:value={auditQ} onchange={() => loadAudit()} /><button onclick={() => loadAudit()}>Filter</button></div>
    <div class="card table-wrap"><table class="data"><thead><tr><th>When</th><th>User</th><th>Action</th><th>Record</th><th>Detail</th><th class="hide-phone">Address</th></tr></thead>
      <tbody>{#each audit as a}<tr><td style="white-space: nowrap">{dateTime(a.at)}</td><td>{a.username}</td><td>{a.action}</td><td class="id">{a.recordId}</td><td class="small mono" style="max-width: 420px; word-break: break-all">{JSON.stringify(a.detail)}</td><td class="hide-phone">{a.ip}</td></tr>{/each}</tbody></table></div>
    {#if audit.length >= 200}<button style="margin-top: 0.75rem" onclick={() => loadAudit(true)}>Older entries</button>{/if}
  {:else if section === 'system'}
    {#if gs}
      <div class="card card-body" style="margin-bottom: 1rem">
        <h3>Google sign-in</h3>
        <p class="small muted">People sign in with their Google account. Only addresses in an allowed domain or listed individually can sign in. New accounts start as Viewer with no space access.</p>
        <div class="grid2">
          <div class="field"><label for="gcid">Client ID</label><input id="gcid" autocomplete="off" bind:value={gs.clientId} /></div>
          <div class="field"><label for="gcs">Client secret</label><input id="gcs" type="password" autocomplete="new-password" placeholder={gs.secretSet ? 'Saved. Enter a new one to replace it.' : ''} bind:value={gSecret} /></div>
          <div class="field"><label for="gpu">Public address</label><input id="gpu" placeholder="https://designhub.example.com" bind:value={gs.publicUrl} /></div>
          <div class="field"><label for="gru">Redirect URI for the Google client</label><input id="gru" readonly value={gs.redirectUri || 'Set the public address first.'} /></div>
          <div class="field"><label for="gd">Allowed Google Workspace domains, one per line</label><textarea id="gd" placeholder="example.com" bind:value={gDomains}></textarea></div>
          <div class="field"><label for="ge">Allowed individual addresses, one per line</label><textarea id="ge" placeholder="someone@gmail.com" bind:value={gEmails}></textarea></div>
        </div>
        <div class="row end"><button class="primary" onclick={() => saveGoogle(googleFields())}>Save</button></div>
        <label class="checkbox" style="margin-top: 0.75rem"><input type="checkbox" checked={gs.enabled} onchange={(e) => { const on = (e.target as HTMLInputElement).checked; saveGoogle({ ...googleFields(), enabled: on }, on ? 'Google sign-in turned on.' : 'Google sign-in turned off.', e.target as HTMLInputElement); }} /> Show Google sign-in</label>
        <label class="checkbox"><input type="checkbox" checked={gs.required} disabled={!gs.enabled || session.me?.libraryRole !== 'Super Admin'} onchange={(e) => { const on = (e.target as HTMLInputElement).checked; if (on && !confirm('Turn off password sign-in for everyone, including Super Admins? Recovery is then only possible with the designhub recovery command on the server.')) { (e.target as HTMLInputElement).checked = false; return; } saveGoogle({ required: on }, on ? 'Password sign-in turned off.' : 'Password sign-in turned on.', e.target as HTMLInputElement); }} /> Turn off password sign-in</label>
        {#if gs.enabled && !gs.meLinked}<p class="small muted">Sign out and sign in with Google once before turning off password sign-in.</p>{/if}
      </div>
    {/if}
    {#if sys}
      <div class="card card-body" style="margin-bottom: 1rem">
        <h3>Allowed addresses</h3>
        <p class="small muted">Host names or IP addresses DesignHub answers to, one per line. Empty allows any address. Requests to other addresses are refused.</p>
        <textarea bind:value={hostsText} placeholder="192.168.1.50"></textarea>
        <div class="row end" style="margin-top: 0.5rem"><button class="primary" onclick={() => saveSys({ allowedHosts: hostsText.split(/\s+/).filter(Boolean) })}>Save</button></div>
      </div>
      <div class="card card-body" style="margin-bottom: 1rem">
        <h3>Change orders</h3>
        <label class="checkbox"><input type="checkbox" checked={sys.coRejectReturnsToDraft} onchange={(e) => saveSys({ coRejectReturnsToDraft: (e.target as HTMLInputElement).checked })} /> When a change order is rejected or cancelled, return its edits to the affected items' drafts</label>
      </div>
      <div class="card card-body" style="margin-bottom: 1rem">
        <h3>Backups</h3>
        <div class="row"><label for="bh" style="margin: 0">Daily at</label><select id="bh" style="width: auto" bind:value={sys.backupSchedule.hour}>{#each Array(24).fill(0).map((_, i) => i) as h}<option value={h}>{String(h).padStart(2, '0')}:00</option>{/each}</select>
          <label for="bk" style="margin: 0">keep</label><input id="bk" type="number" min="1" max="365" style="width: 8ch" bind:value={sys.backupSchedule.keep} /><span>copies</span>
          <button onclick={() => saveSys({ backupSchedule: { hour: Number(sys.backupSchedule.hour), keep: Number(sys.backupSchedule.keep) } })}>Save</button>
          <span class="spacer"></span><button class="primary" disabled={backingUp} onclick={backupNow}>{backingUp ? 'Backing up...' : 'Back up now'}</button></div>
        {#if status}<table class="data" style="margin-top: 0.75rem"><tbody>{#each status.backups as b}<tr><td>{b.name}</td><td>{b.error || 'Complete'}</td><td class="num">{bytes(b.size)}</td></tr>{:else}<tr><td class="empty">No backups yet.</td></tr>{/each}</tbody></table>{/if}
      </div>
    {/if}
    {#if status}
      <div class="card card-body">
        <h3>Status</h3>
        <dl class="props">
          <dt>Version</dt><dd>{status.version}</dd>
          <dt>Database</dt><dd>{status.database.connected ? 'Connected' : 'Not connected'}{#if status.database.server}<br /><span class="small muted">{status.database.server}</span>{/if}</dd>
          <dt>Database size</dt><dd>{bytes(status.database.sizeBytes || 0)}</dd>
          <dt>Migrations</dt><dd class="small">{(status.database.migrations || []).join(', ')}</dd>
          <dt>Repository head</dt><dd class="mono small">{status.repository.head}</dd>
          <dt>Baselines</dt><dd>{status.repository.baselines}</dd>
          <dt>Records</dt><dd class="small">{Object.entries(status.records).map(([k, n]) => `${k} ${n}`).join(', ')}</dd>
        </dl>
        <div class="row" style="margin-top: 0.75rem"><button onclick={async () => { await post('/api/admin/reindex'); toast('Search index rebuilt.'); }}>Rebuild search index</button>
          <a class="btn" href="/api/admin/signing-key" download>Download baseline signing key (public)</a></div>
      </div>
    {/if}
  {/if}
</div>

{#if newUser}
  <Modal title="New user" onclose={() => (newUser = null)}>
    <div class="field"><label for="nu">Username</label><input id="nu" bind:value={newUser.username} autocomplete="off" /></div>
    <div class="field"><label for="nd">Display name</label><input id="nd" bind:value={newUser.displayName} /></div>
    <div class="field"><label for="ne">Email</label><input id="ne" bind:value={newUser.email} /></div>
    <div class="field"><label for="np">Password (at least 10 characters)</label><input id="np" type="password" autocomplete="new-password" bind:value={newUser.password} /></div>
    <div class="field"><label for="nr">Library role</label><select id="nr" bind:value={newUser.libraryRole}>{#each roles as r}{#if r.name !== 'Super Admin' || session.me?.libraryRole === 'Super Admin'}<option>{r.name}</option>{/if}{/each}</select></div>
    {#each spaces as s}<div class="field"><label for="sr-{s.id}">Role in {s.name}</label><select id="sr-{s.id}" bind:value={newUser.spaceRoles[s.id]}><option value="">No access</option>{#each roles as r}<option>{r.name}</option>{/each}</select></div>{/each}
    {#snippet footer()}<button onclick={() => (newUser = null)}>Cancel</button><button class="primary" onclick={createUser}>Create</button>{/snippet}
  </Modal>
{/if}
{#if newSpace}
  <Modal title="New space" onclose={() => (newSpace = null)}>
    <div class="field"><label for="ns">Name</label><input id="ns" maxlength="60" bind:value={newSpace.name} /></div>
    <div class="field"><label for="nsd">Description</label><input id="nsd" bind:value={newSpace.description} /></div>
    {#snippet footer()}<button onclick={() => (newSpace = null)}>Cancel</button><button class="primary" onclick={createSpace}>Create</button>{/snippet}
  </Modal>
{/if}
