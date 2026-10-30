<script lang="ts">
  import { get, post, patch, put, del } from '../lib/api';
  import { session, toast, onEvent, userName, spaceName } from '../lib/state.svelte';
  import { navigate, route, setQuery, recordPath } from '../lib/router.svelte';
  import { dateTime, date, kindLabel } from '../lib/format';
  import Status from '../components/Status.svelte';
  import Menu from '../components/Menu.svelte';
  import DiffView from '../components/DiffView.svelte';
  import Comments from '../components/Comments.svelte';
  import SignatureDialog from '../components/SignatureDialog.svelte';
  import RecordPicker from '../components/RecordPicker.svelte';
  import UserPicker from '../components/UserPicker.svelte';
  import LockDialog from '../components/LockDialog.svelte';
  import Modal from '../components/Modal.svelte';
  import CoHistory from '../components/CoHistory.svelte';
  import { onDestroy } from 'svelte';

  let { params }: { params: any; pattern?: string } = $props();
  const id = params.id;
  let d: any = $state(null);
  let error = $state('');
  let tab = $state(route.query.get('tab') || 'details');
  let changes: any[] = $state([]);
  let side = $state(typeof window !== 'undefined' && window.innerWidth > 700);
  let editing = $state(false);
  let form: any = $state({});
  let sign: '' | 'Approved' | 'Rejected' | 'Release' = $state('');
  let addAction = $state('');
  let approverDialog = $state(false);
  let approvers: string[] = $state([]);
  let lockOpen = $state(false);

  async function load() {
    try {
      d = await get('/api/change-orders/' + id);
      error = '';
      if (tab === 'changes') loadChanges();
    } catch (e: any) { error = e.status === 404 ? 'not found' : e.message; }
  }
  load();
  async function loadChanges() { changes = await get(`/api/change-orders/${id}/changes`); }
  const off = onEvent((e) => { if (e.type === 'record' && e.id === id && !editing) load(); });
  onDestroy(() => off());
  function setTab(t: string) { tab = t; setQuery({ tab: t === 'details' ? null : t }); if (t === 'changes') loadChanges(); }

  function startEdit() {
    form = { title: d.co.title, changeType: d.co.changeType, reason: d.co.reason, description: d.co.description, stockDisposition: d.co.stockDisposition,
      needDate: d.co.needDate ? d.co.needDate.slice(0, 10) : '', priority: d.co.priority };
    editing = true;
  }
  async function saveEdit() {
    try { await patch('/api/change-orders/' + id, form); editing = false; toast('Saved.'); load(); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  async function act(path: string, msg: string, body: any = {}) {
    try { await post(`/api/change-orders/${id}/${path}`, body); toast(msg); load(); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  async function decide(password: string, comment: string) {
    if (sign === 'Release') await post(`/api/change-orders/${id}/release`, { password, comment });
    else await post(`/api/change-orders/${id}/decision`, { decision: sign, password, comment });
    toast(sign === 'Release' ? `${id} released.` : sign === 'Approved' ? 'Approval recorded.' : 'Rejection recorded.');
    sign = '';
    load();
  }
  async function addItem(item: string) {
    try { await post(`/api/change-orders/${id}/items`, { item, action: addAction }); load(); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  async function removeItem(item: string) {
    if (!confirm(`Remove ${item} from ${id}? Edits made under the change order return to the item's draft.`)) return;
    try { await del(`/api/change-orders/${id}/items/${encodeURIComponent(item)}`); load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  function openApprovers() { approvers = d.approvers.map((a: any) => a.userId); approverDialog = true; }
  async function saveApprovers() {
    try { await put(`/api/change-orders/${id}/approvers`, { approvers }); approverDialog = false; load(); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  async function watch() {
    try { await post(`/api/records/${id}/watch`, { watch: !d.watching }); d.watching = !d.watching; } catch {}
  }
  const errors = $derived((d?.issues || []).filter((i: any) => i.level === 'error'));
</script>

{#if error === 'not found'}
  <div class="content narrow"><div class="card card-body empty"><h2>Not found</h2><p>{id} does not exist, or you do not have access to it.</p></div></div>
{:else if error}<div class="content narrow"><div class="error-box">{error}</div></div>
{:else if d}
<div class="content" style="padding-bottom: {d.perms.decide ? '5rem' : '0'}">
  <div class="record-head">
    <div class="titles">
      <div class="meta"><span class="id">{id}</span><Status value={d.co.state} /><span class="muted">{d.co.changeType}</span>{#if d.lock.length}<span class="pill">🔒</span>{/if}</div>
      <h1>{d.co.title}</h1>
    </div>
    <div class="row">
      <button onclick={() => (session.commentsOpen = !session.commentsOpen)}>Comments</button>
      {#if d.perms.submit}<button class="primary" onclick={() => act('submit', 'Submitted for review.')}>Submit for review</button>{/if}
      {#if d.perms.release}<button class="primary" onclick={() => (sign = 'Release')}>Release</button>{/if}
      <Menu>
        {#if d.perms.edit}<button onclick={startEdit}>Edit</button>{/if}
        {#if d.perms.manageApprovers}<button onclick={openApprovers}>Add or remove approvers</button>{/if}
        {#if d.perms.withdraw}<button onclick={() => act('withdraw', 'Returned to Draft.')}>Return to Draft</button>{/if}
        <button onclick={watch}>{d.watching ? 'Stop watching' : 'Watch'}</button>
        {#if d.perms.lock}<button onclick={() => (lockOpen = true)}>Lock</button>{/if}
        {#if d.perms.cancel}<hr /><button onclick={() => confirm(`Cancel ${id}?`) && act('cancel', 'Cancelled.')}>Cancel change order</button>{/if}
      </Menu>
    </div>
  </div>
  {#if d.co.state === 'Released'}<div class="banner info">Released by {d.co.releasedBy} on {dateTime(d.co.releasedAt)}. Baseline <a href="/baselines">{d.co.baseline}</a>.</div>{/if}
  {#if errors.length && d.co.state !== 'Released'}<div class="banner warn">{errors.length} validation error{errors.length > 1 ? 's' : ''} must be fixed before approval. <a href="?tab=validation" onclick={(e) => { e.preventDefault(); setTab('validation'); }}>Show</a></div>{/if}

  <div class="tabs">
    {#each [['details', 'Details'], ['items', `Affected items (${d.items.length})`], ['changes', 'Changes'], ['approvals', 'Approvals'], ['validation', 'Validation'], ['history', 'History']] as [k, l]}<button class:active={tab === k} onclick={() => setTab(k)}>{l}</button>{/each}
  </div>

  {#if tab === 'details'}
    <div class="card card-body">
      {#if editing}
        <div class="grid2">
          <div class="field"><label for="ft">Title</label><input id="ft" maxlength="100" bind:value={form.title} /></div>
          <div class="field"><label for="fct">Change type</label><select id="fct" bind:value={form.changeType}>{#each ['Initial release', 'Design change', 'Documentation only', 'Obsolescence'] as t}<option>{t}</option>{/each}</select></div>
          <div class="field"><label for="fp">Priority</label><select id="fp" bind:value={form.priority}>{#each ['Low', 'Normal', 'High', 'Urgent'] as t}<option>{t}</option>{/each}</select></div>
          <div class="field"><label for="fn">Need date</label><input id="fn" type="date" bind:value={form.needDate} /></div>
          <div class="field"><label for="fs">Existing stock disposition</label><select id="fs" bind:value={form.stockDisposition}>{#each ['', 'Use as is', 'Rework', 'Scrap', 'Return to vendor'] as t}<option value={t}>{t || 'None'}</option>{/each}</select></div>
        </div>
        <div class="field"><label for="fr">Reason for change</label><textarea id="fr" bind:value={form.reason}></textarea></div>
        <div class="field"><label for="fd">Description of change</label><textarea id="fd" bind:value={form.description}></textarea></div>
        <div class="row end"><button onclick={() => (editing = false)}>Cancel</button><button class="primary" onclick={saveEdit}>Save</button></div>
      {:else}
        <dl class="props">
          <dt>Reason for change</dt><dd style="white-space: pre-wrap">{d.co.reason || '-'}</dd>
          <dt>Description of change</dt><dd style="white-space: pre-wrap">{d.co.description || '-'}</dd>
          <dt>Priority</dt><dd>{d.co.priority}</dd>
          <dt>Need date</dt><dd>{date(d.co.needDate?.slice(0, 10)) || '-'}</dd>
          <dt>Existing stock disposition</dt><dd>{d.co.stockDisposition || '-'}</dd>
          <dt>Owner</dt><dd>{d.co.owner}</dd>
          <dt>Created by</dt><dd>{d.co.createdBy} on {dateTime(d.co.createdAt)}</dd>
          <dt>Home space</dt><dd>{spaceName(d.co.space)}</dd>
          <dt>Fixes</dt><dd>{#each d.fixes || [] as f}<a class="id" href={recordPath(f)}>{f}</a> {:else}-{/each}</dd>
          <dt>Branch</dt><dd class="mono small">{d.co.branch}</dd>
        </dl>
      {/if}
    </div>
  {:else if tab === 'items'}
    <div class="card table-wrap">
      <table class="data"><thead><tr><th>Item</th><th>Name</th><th>Action</th><th>From</th><th>To</th><th class="hide-phone">Current status</th><th></th></tr></thead><tbody>
        {#each d.items as it}<tr><td>{#if it.hidden}<span class="muted">Restricted item</span>{:else}<a class="id" href={recordPath(it.item, it.kind)}>{it.item}</a>{/if}</td><td>{it.title}</td><td>{it.action}</td><td>{it.fromRev}</td><td>{it.toRev}</td><td class="hide-phone"><Status value={it.status} /></td>
          <td>{#if d.perms.edit && !it.hidden}<button class="small danger" onclick={() => removeItem(it.item)}>Remove</button>{/if}</td></tr>
        {:else}<tr><td colspan="7" class="empty">No affected items yet.</td></tr>{/each}
      </tbody></table>
    </div>
    {#if d.perms.edit}
      <div class="card card-body row" style="margin-top: 1rem; align-items: flex-end">
        <div style="width: 180px"><label for="aa">Action</label><select id="aa" bind:value={addAction}><option value="">Automatic</option><option>Release</option><option>Revise</option><option>Obsolete</option></select></div>
        <div style="flex: 1; min-width: 240px"><label for="ai">Add item or controlled page</label><RecordPicker kinds="part,assembly,requirement,procedure,document,page" onpick={(i) => addItem(i)} /></div>
      </div>
      <p class="small muted">Edit an affected item from its own page; while the change order is in Draft, edits are saved on the change order.</p>
    {/if}
  {:else if tab === 'changes'}
    <div class="row" style="margin-bottom: 0.75rem"><span class="muted small">Released data on the left, proposed on the right.</span><span class="spacer"></span>
      <button class="small" class:primary={!side} onclick={() => (side = false)}>Single column</button><button class="small hide-phone" class:primary={side} onclick={() => (side = true)}>Side by side</button></div>
    {#each changes as c}
      <div class="card" style="margin-bottom: 1rem">
        <div class="card-head"><a class="id" href={recordPath(c.item)}>{c.item}</a> {c.title}<span class="spacer"></span><span class="pill">{c.action}{c.fromRev ? ' ' + c.fromRev : ''} → {c.toRev}</span></div>
        <div class="card-body">
          {#if c.fields.length}
            <table class="data" style="margin-bottom: 0.75rem"><thead><tr><th>Field</th><th>Released</th><th>Proposed</th></tr></thead><tbody>
              {#each c.fields.filter((f: any) => f.field !== 'Body') as f}<tr><td>{f.field}</td><td style="white-space: pre-wrap" class="small">{f.from || '-'}</td><td style="white-space: pre-wrap" class="small">{f.to || '-'}</td></tr>{/each}
            </tbody></table>
          {/if}
          <details><summary class="small">Full file comparison</summary><DiffView lines={c.lines} mode={side && window.innerWidth > 700 ? 'side' : 'single'} /></details>
        </div>
      </div>
    {:else}<div class="card empty">No changes.</div>{/each}
  {:else if tab === 'approvals'}
    <div class="card table-wrap">
      <table class="data"><thead><tr><th>Approver</th><th>Decision</th><th>Time</th><th>Comment</th></tr></thead><tbody>
        {#each d.approvers as a}<tr><td>{a.name}</td><td>{#if a.decision}<Status value={a.decision} />{:else}<span class="muted">Pending</span>{/if}</td><td>{dateTime(a.decidedAt)}</td><td style="white-space: pre-wrap">{a.comment}</td></tr>
        {:else}<tr><td colspan="4" class="empty">No approvers yet.</td></tr>{/each}
      </tbody></table>
    </div>
    {#if d.perms.manageApprovers}<button style="margin-top: 0.75rem" onclick={openApprovers}>Add approver</button>{/if}
    <p class="small muted">Every approver must approve. One rejection rejects the change order. The author cannot be the only approver.</p>
  {:else if tab === 'validation'}
    <div class="card">
      {#each d.issues as is}<div class="comment"><span class="issue-dot {is.level}"></span><strong>{is.level === 'error' ? 'Error' : 'Warning'}</strong> {#if is.item}<a class="id" href={recordPath(is.item)}>{is.item}</a>{/if} {is.message}</div>
      {:else}<div class="empty">No validation issues.</div>{/each}
    </div>
  {:else if tab === 'history'}
    <CoHistory {id} />
  {/if}
</div>

{#if d.perms.decide}
  <div class="decision-bar">
    <span class="hide-phone">Your decision on {id}</span><span class="spacer"></span>
    <button class="danger" onclick={() => (sign = 'Rejected')}>Reject</button>
    <button class="primary" onclick={() => (sign = 'Approved')}>Approve</button>
  </div>
{/if}
{#if sign}<SignatureDialog title={sign === 'Release' ? `Release ${id}` : sign === 'Approved' ? `Approve ${id}` : `Reject ${id}`} decision={sign} requireComment={sign === 'Rejected'} onsign={decide} onclose={() => (sign = '')} />{/if}
{#if session.commentsOpen}<Comments recordId={id} />{/if}
{#if lockOpen}<LockDialog recordId={id} current={d.lock} onclose={() => (lockOpen = false)} ondone={() => { lockOpen = false; load(); }} />{/if}
{#if approverDialog}
  <Modal title="Approvers" onclose={() => (approverDialog = false)}>
    <UserPicker multiple bind:value={approvers} placeholder="Add an approver" filter={(u) => ['Super Admin', 'Release Authority', 'Approver'].includes(u.libraryRole)} />
    {#snippet footer()}<button onclick={() => (approverDialog = false)}>Cancel</button><button class="primary" onclick={saveApprovers}>Save</button>{/snippet}
  </Modal>
{/if}
{/if}
