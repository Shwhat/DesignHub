<script lang="ts">
  import { untrack, onDestroy } from 'svelte';
  import { get, patch, post } from '../lib/api';
  import { session, toast, onEvent, can } from '../lib/state.svelte';
  import { route, setQuery, navigate } from '../lib/router.svelte';
  import { dateTime, date } from '../lib/format';
  import Status from '../components/Status.svelte';
  import Menu from '../components/Menu.svelte';
  import Comments from '../components/Comments.svelte';
  import RecordPicker from '../components/RecordPicker.svelte';
  import Files from '../components/Files.svelte';
  import Markdown from '../components/Markdown.svelte';

  let { params }: { params: any; pattern?: string } = $props();
  const id = untrack(() => params.id);
  let d: any = $state(null);
  let error = $state('');
  let tab = $state(route.query.get('tab') || 'details');
  let editing = $state(false);
  let c: any = $state(null);
  let mfrs: any[] = $state([]);
  const kinds = ['Vendor', 'Contractor', 'Customer', 'Other'];

  async function load() {
    try {
      d = await get('/api/companies/' + id);
      error = '';
    } catch (e: any) {
      error = e.status === 404 ? 'not found' : e.message;
    }
  }
  load();
  const off = onEvent((e) => { if (e.type === 'record' && e.id === id && !editing) load(); });
  onDestroy(off);
  function setTab(t: string) { tab = t; setQuery({ tab: t }); }
  async function startEdit() {
    c = JSON.parse(JSON.stringify(d.company));
    if (!mfrs.length) mfrs = await get('/api/manufacturers').catch(() => []);
    editing = true;
    setTab('details');
  }
  async function save() {
    try {
      await patch('/api/companies/' + id, { ...c, version: d.company.version });
      editing = false;
      toast('Saved.');
      load();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  async function setActive(active: boolean) {
    if (!active && !confirm(`Deactivate ${d.company.name}? Its Vendor and Contractor accounts can no longer sign in.`)) return;
    try { await patch('/api/companies/' + id, { active, version: d.company.version }); load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function addSupplied(itemId: string) {
    try { await post(`/api/companies/${id}/supplied`, { add: [itemId] }); load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function removeSupplied(itemId: string) {
    try { await post(`/api/companies/${id}/supplied`, { remove: [itemId] }); load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function watch() { try { await post(`/api/records/${id}/watch`, { watch: !d.watching }); d.watching = !d.watching; } catch {} }
  function describe(ch: any) {
    return Object.entries(ch || {}).map(([k, v]: any) => k === 'created' ? 'Created' : k === 'import' ? 'Updated by CSV import' : k === 'supplied' ? `Supplied items: added ${(v.added || []).join(', ') || 'none'}, removed ${(v.removed || []).join(', ') || 'none'}` : `${k} changed`).join('; ');
  }
  const statusClass = (s: string) => (s === 'Approved' ? 'Approved' : s === 'Conditional' ? 'In-Review' : s ? 'Rejected' : '');
</script>

{#if error === 'not found'}
  <div class="content narrow"><div class="card card-body empty"><h2>Not found</h2><p>{id} does not exist, or you do not have access to it.</p></div></div>
{:else if error}<div class="content narrow"><div class="error-box">{error}</div></div>
{:else if d}
<div class="content narrow">
  <nav class="small muted" style="margin-bottom: 4px"><a href="/library/company">Companies</a></nav>
  <div class="record-head">
    <div class="titles">
      <div class="meta"><span class="id">{id}</span>{#if d.company.supplierStatus}<span class="status {statusClass(d.company.supplierStatus)}">{d.company.supplierStatus}</span>{/if}{#if !d.company.active}<span class="status Cancelled">Inactive</span>{/if}<span class="muted">{d.company.kinds.join(', ')}</span></div>
      <h1>{d.company.name}</h1>
    </div>
    <div class="row">
      <button onclick={() => (session.commentsOpen = !session.commentsOpen)}>Comments</button>
      {#if d.canEdit && !editing}<button class="primary" onclick={startEdit}>Edit</button>{/if}
      <Menu>
        {#if d.canEdit}<button onclick={startEdit}>Edit</button><button onclick={() => setActive(!d.company.active)}>{d.company.active ? 'Deactivate...' : 'Activate'}</button>{/if}
        {#if d.canManagePeople}<button onclick={() => navigate('/admin/users?company=' + id)}>Add a Vendor or Contractor user...</button>{/if}
        <button onclick={watch}>{d.watching ? 'Stop watching' : 'Watch'}</button>
      </Menu>
    </div>
  </div>
  {#if d.company.supplierStatus && d.company.supplierStatus !== 'Approved'}<div class="banner warn">Supplier status: {d.company.supplierStatus}.</div>{/if}
  {#if editing}
    <div class="save-bar"><strong>Editing</strong><span class="spacer"></span><button onclick={() => (editing = false)}>Cancel</button><button class="primary" onclick={save}>Save</button></div>
  {/if}
  <div class="tabs">
    {#each [['details', 'Details'], ['supplied', `Supplied items (${d.supplied.length})`], ['ncrs', `NCRs (${d.ncrs.length})`], ['people', `People (${d.people.length})`], ['attachments', 'Attachments'], ['history', 'History']] as [k, l]}
      <button class:active={tab === k} onclick={() => setTab(k)}>{l}</button>
    {/each}
  </div>

  {#if tab === 'details'}
    <div class="card card-body">
      {#if editing}
        <div class="field"><label for="c-n">Name</label><input id="c-n" maxlength="100" bind:value={c.name} /></div>
        <div class="field"><span class="lbl">Kind</span><div class="row">{#each kinds as k}<label class="checkbox"><input type="checkbox" checked={c.kinds.includes(k)} onchange={(e) => (c.kinds = (e.target as HTMLInputElement).checked ? [...c.kinds, k] : c.kinds.filter((x: string) => x !== k))} /> {k}</label>{/each}</div></div>
        <div class="grid2">
          <div class="field"><label for="c-s">Supplier status</label><select id="c-s" bind:value={c.supplierStatus}><option value="">Not set</option><option>Approved</option><option>Conditional</option><option>Not approved</option></select></div>
          <div class="field"><label for="c-m">Manufacturer codes</label><select id="c-m" multiple size="4" bind:value={c.manufacturerCodes}>{#each mfrs as m}<option value={m.code}>{m.code} {m.name}</option>{/each}</select></div>
          <div class="field"><label for="c-p">Phone</label><input id="c-p" bind:value={c.phone} /></div>
          <div class="field"><label for="c-w">Website</label><input id="c-w" bind:value={c.website} /></div>
        </div>
        <div class="field"><label for="c-a">Address</label><textarea id="c-a" rows="3" bind:value={c.address}></textarea></div>
        <div class="field"><span class="lbl">Contacts</span>
          {#each c.contacts as ct, i}
            <div class="row" style="margin-bottom: 4px"><input placeholder="Name" bind:value={ct.name} style="flex: 2" /><input placeholder="Role" bind:value={ct.role} style="flex: 1" /><input placeholder="Email" bind:value={ct.email} style="flex: 2" /><input placeholder="Phone" bind:value={ct.phone} style="flex: 1" /><button class="small ghost danger" onclick={() => c.contacts.splice(i, 1)}>Remove</button></div>
          {/each}
          <button class="small" onclick={() => c.contacts.push({ name: '', role: '', email: '', phone: '' })}>Add contact</button>
        </div>
        <div class="field"><label for="c-no">Notes</label><textarea id="c-no" rows="4" bind:value={c.notes}></textarea></div>
      {:else}
        <dl class="props">
          <dt>Kind</dt><dd>{d.company.kinds.join(', ')}</dd>
          <dt>Supplier status</dt><dd>{d.company.supplierStatus || '-'}</dd>
          <dt>Manufacturer codes</dt><dd>{d.company.manufacturerCodes.join(', ') || '-'}</dd>
          <dt>Address</dt><dd style="white-space: pre-wrap">{d.company.address || '-'}</dd>
          <dt>Phone</dt><dd>{d.company.phone || '-'}</dd>
          <dt>Website</dt><dd>{d.company.website || '-'}</dd>
          <dt>Active</dt><dd>{d.company.active ? 'Yes' : 'No'}</dd>
        </dl>
        <h3 style="margin-top: 12px">Contacts</h3>
        {#if d.company.contacts.length}
          <table class="data"><thead><tr><th>Name</th><th>Role</th><th>Email</th><th>Phone</th></tr></thead><tbody>
            {#each d.company.contacts as ct}<tr><td>{ct.name}</td><td>{ct.role}</td><td>{ct.email}</td><td>{ct.phone}</td></tr>{/each}
          </tbody></table>
        {:else}<div class="muted small">No contacts.</div>{/if}
        <h3 style="margin-top: 12px">Notes</h3>
        {#if d.company.notes}<Markdown source={d.company.notes} />{:else}<div class="muted small">No notes.</div>{/if}
      {/if}
    </div>
  {:else if tab === 'supplied'}
    <div class="card">
      {#if d.canEdit || can('library.edit')}
        <div class="card-body" style="border-bottom: 1px solid var(--line2)"><label for="c-add">Add a supplied item</label><RecordPicker kinds="part,assembly" onpick={addSupplied} /></div>
      {/if}
      <div class="table-wrap"><table class="data"><thead><tr><th>ID</th><th>Rev</th><th>Name</th><th>Status</th><th></th></tr></thead><tbody>
        {#each d.supplied as s}<tr><td><a class="id" href="/items/{encodeURIComponent(s.id)}">{s.id}</a></td><td>{s.revision}</td><td>{s.title}</td><td><Status value={s.status} /></td>
          <td>{#if d.canEdit || can('library.edit')}<button class="small ghost danger" onclick={() => removeSupplied(s.id)}>Remove</button>{/if}</td></tr>
        {:else}<tr><td colspan="5" class="empty">No supplied items.</td></tr>{/each}
      </tbody></table></div>
    </div>
  {:else if tab === 'ncrs'}
    <div class="card table-wrap"><table class="data"><thead><tr><th>ID</th><th>Title</th><th>Status</th></tr></thead><tbody>
      {#each d.ncrs as n}<tr class="clickable" onclick={() => navigate('/work/' + n.id)}><td><span class="id">{n.id}</span></td><td>{n.title}</td><td><Status value={n.status} /></td></tr>
      {:else}<tr><td colspan="3" class="empty">No nonconformances name this supplier.</td></tr>{/each}
    </tbody></table></div>
  {:else if tab === 'people'}
    <div class="card table-wrap"><table class="data"><thead><tr><th>Name</th><th>Role</th><th>Account expiry</th><th>Active</th></tr></thead><tbody>
      {#each d.people as p}<tr class:clickable={d.canManagePeople} onclick={() => d.canManagePeople && navigate('/admin/users/' + p.id)}><td>{p.displayName}</td><td>{p.role}</td><td>{date(p.expiresOn)}</td><td>{p.active ? 'Yes' : 'No'}</td></tr>
      {:else}<tr><td colspan="4" class="empty">No accounts are linked to this company.</td></tr>{/each}
    </tbody></table></div>
  {:else if tab === 'attachments'}
    {#if editing}<Files recordId={id} bind:files={c.attachments} editing />{:else}<Files recordId={id} files={d.company.attachments} />{/if}
    {#if !editing && d.canEdit}<p class="small muted">Edit the company to add or remove attachments. Company attachments are never shown to Vendors.</p>{/if}
  {:else if tab === 'history'}
    <div class="card table-wrap"><table class="data"><thead><tr><th>When</th><th>Who</th><th>Change</th></tr></thead><tbody>
      {#each d.history as h}<tr><td style="white-space: nowrap">{dateTime(h.at)}</td><td>{h.user}</td><td class="small">{describe(h.changes)}</td></tr>{/each}
    </tbody></table></div>
  {/if}
</div>
{#if session.commentsOpen}<Comments recordId={id} />{/if}
{/if}

<style>
  .lbl { font-size: 11.5px; font-weight: 600; color: var(--text-2); display: block; margin-bottom: 3px; }
</style>
