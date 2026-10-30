<script lang="ts">
  import { get, patch, post, ApiError } from '../lib/api';
  import { session, toast, onEvent, setSpace, spaceName } from '../lib/state.svelte';
  import { navigate, route, setQuery, recordPath, setLeaveGuard } from '../lib/router.svelte';
  import { dateTime, kindLabel } from '../lib/format';
  import Status from '../components/Status.svelte';
  import Menu from '../components/Menu.svelte';
  import WorkFields from '../components/WorkFields.svelte';
  import Comments from '../components/Comments.svelte';
  import SignatureDialog from '../components/SignatureDialog.svelte';
  import LockDialog from '../components/LockDialog.svelte';
  import RecordPicker from '../components/RecordPicker.svelte';
  import Files from '../components/Files.svelte';
  import { onDestroy } from 'svelte';

  let { params }: { params: any; pattern?: string } = $props();
  const id = params.id;
  let d: any = $state(null);
  let error = $state('');
  let tab = $state(route.query.get('tab') || '');
  let editing = $state(false);
  let w: any = $state(null);
  let sign = $state('');
  let lockOpen = $state(false);
  let linkType = $state('relates to');

  async function load() {
    try {
      d = await get('/api/work/' + id);
      error = '';
      setSpace(d.item.space);
      if (!tab) tab = d.item.type === 'ncr' ? 'report' : 'details';
    } catch (e: any) { error = e.status === 404 ? 'not found' : e.message; }
  }
  load();
  const off = onEvent((e) => { if ((e.type === 'record' || e.type === 'board') && e.id === id && !editing) load(); });
  onDestroy(() => { off(); setLeaveGuard(null); });
  const isNCR = $derived(d?.item.type === 'ncr');
  const tabs = $derived(isNCR
    ? [['report', 'Report'], ['disposition', 'Disposition'], ['corrective', 'Corrective action'], ['verification', 'Verification'], ['links', 'Links'], ['attachments', 'Attachments'], ['history', 'History']]
    : [['details', 'Details'], ['links', 'Links'], ['attachments', 'Attachments'], ['history', 'History']]);
  function setTab(t: string) { tab = t; setQuery({ tab: t }); }
  function startEdit() {
    w = JSON.parse(JSON.stringify(d.item));
    editing = true;
    setLeaveGuard(() => confirm('Leave without saving?'));
  }
  async function save() {
    try {
      await patch('/api/work/' + id, { version: d.item.version, title: w.title, description: w.description, priority: w.priority, assigneeId: w.assigneeId || '',
        dueDate: w.dueDate || '', epic: w.epic, labels: w.labels, fields: w.fields, attachments: w.attachments });
      editing = false; setLeaveGuard(null); toast('Saved.'); load();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  async function setStatus(s: string) {
    try { await post(`/api/work/${id}/status`, { status: s }); toast(`${id} is now ${s}.`); load(); }
    catch (e: any) { toast(e.message, 'error'); load(); }
  }
  async function signNCR(password: string, comment: string) {
    await post(`/api/work/${id}/sign`, { decision: sign, password, comment });
    sign = ''; toast('Signature recorded.'); load();
  }
  async function addLink(to: string) {
    try { await post(`/api/work/${id}/links`, { add: [{ type: linkType, to }] }); load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function removeLink(l: any) {
    try { await post(`/api/work/${id}/links`, { remove: [{ type: l.type, to: l.id }] }); load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function createCO() {
    try { const r = await post(`/api/work/${id}/create-co`); toast(`Created ${r.id}.`); navigate('/change-orders/' + r.id); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function watch() { try { await post(`/api/records/${id}/watch`, { watch: !d.watching }); d.watching = !d.watching; } catch {} }
  function describe(ch: any) {
    return Object.entries(ch || {}).map(([k, v]: any) => k === 'created' ? 'Created' : k === 'links' ? 'Links changed' : `${k}: ${fmt(v?.from)} to ${fmt(v?.to)}`).join('; ');
  }
  const fmt = (x: any) => (x === null || x === undefined || x === '' ? '(empty)' : typeof x === 'object' ? JSON.stringify(x) : String(x).length > 60 ? String(x).slice(0, 60) + '...' : String(x));
  const sectionEditable = (t: string) => ['details', 'report', 'disposition', 'corrective', 'verification', 'attachments'].includes(t);
</script>

{#if error === 'not found'}
  <div class="content narrow"><div class="card card-body empty"><h2>Not found</h2><p>{id} does not exist, or you do not have access to it.</p></div></div>
{:else if error}<div class="content narrow"><div class="error-box">{error}</div></div>
{:else if d}
<div class="content narrow" style="padding-bottom: {d.canSign ? '5rem' : '0'}">
  <nav class="small muted" style="margin-bottom: 0.4rem"><a href="/spaces/{d.item.space}">{spaceName(d.item.space)}</a> / {kindLabel(d.item.type)}</nav>
  <div class="record-head">
    <div class="titles">
      <div class="meta">
        <span class="id">{id}</span>
        {#if d.canEdit && d.transitions.length}
          <select style="width: auto; min-height: 30px; padding: 0.1rem 0.4rem" aria-label="Status" value={d.item.status} onchange={(e) => setStatus((e.target as HTMLSelectElement).value)}>
            <option value={d.item.status}>{d.item.status}</option>
            {#each d.transitions as t}<option value={t}>Move to {t}</option>{/each}
          </select>
        {:else}<Status value={d.item.status} />{/if}
        {#if d.lock.length}<span class="pill">🔒</span>{/if}
      </div>
      <h1>{d.item.title}</h1>
      {#if d.progress}<div class="row small" style="margin-top: 0.4rem"><div class="progress" style="width: 200px"><div style="width: {d.progress.percent}%"></div></div>{d.progress.percent}% ({d.progress.done} of {d.progress.total} done)</div>{/if}
    </div>
    <div class="row">
      <button onclick={() => (session.commentsOpen = !session.commentsOpen)}>Comments</button>
      {#if d.canEdit && !editing && sectionEditable(tab)}<button class="primary" onclick={startEdit}>Edit</button>{/if}
      <Menu>
        {#if d.canEdit}<button onclick={startEdit}>Edit</button>{/if}
        {#if isNCR}<button onclick={createCO}>Create change order</button>{/if}
        <button onclick={watch}>{d.watching ? 'Stop watching' : 'Watch'}</button>
        <button onclick={() => (lockOpen = true)}>Lock</button>
      </Menu>
    </div>
  </div>
  {#if editing}
    <div class="save-bar"><strong>Editing</strong><span class="spacer"></span><button onclick={() => { editing = false; setLeaveGuard(null); }}>Cancel</button><button class="primary" onclick={save}>Save</button></div>
  {/if}
  <div class="tabs">{#each tabs as [k, l]}<button class:active={tab === k} onclick={() => setTab(k)}>{l}</button>{/each}</div>

  {#if tab === 'links'}
    <div class="card">
      {#each d.links as l}<div class="comment row"><span class="muted">{l.type}</span><a class="id" href={recordPath(l.id, l.kind)}>{l.id}</a>{#if l.rev}<span>rev {l.rev}</span>{/if}<span>{l.title}</span><Status value={l.status} /><span class="spacer"></span>{#if l.out && d.canEdit && ['blocks', 'duplicates', 'relates to', 'affects', 'fixed by'].includes(l.type)}<button class="small danger" onclick={() => removeLink(l)}>Remove</button>{/if}</div>
      {:else}<div class="empty">No links.</div>{/each}
      {#if d.children?.length}<div class="card-head"><h3>Items in this epic</h3></div>{#each d.children as c}<div class="comment row"><a class="id" href="/work/{c.id}">{c.id}</a><Status value={c.status} /></div>{/each}{/if}
      {#if d.canEdit}
        <div class="card-body row" style="align-items: flex-end">
          <div style="width: 160px"><label for="lt">Link type</label><select id="lt" bind:value={linkType}>{#each ['blocks', 'duplicates', 'relates to', 'affects', 'fixed by'] as t}<option>{t}</option>{/each}</select></div>
          <div style="flex: 1; min-width: 220px"><label for="lp">Record</label><RecordPicker kinds={linkType === 'affects' ? 'part,assembly,requirement,procedure,document' : linkType === 'fixed by' ? 'change_order' : linkType === 'relates to' ? '' : 'task,issue,action,epic,ncr'} onpick={addLink} /></div>
        </div>
      {/if}
    </div>
  {:else if tab === 'attachments'}
    {#if editing}<Files recordId={id} bind:files={w.attachments} editing />{:else}<Files recordId={id} files={d.item.attachments} />{/if}
  {:else if tab === 'history'}
    <div class="card table-wrap"><table class="data"><thead><tr><th>When</th><th>Who</th><th>Change</th></tr></thead><tbody>
      {#each d.history as h}<tr><td style="white-space: nowrap">{dateTime(h.at)}</td><td>{h.user}</td><td class="small">{describe(h.changes)}</td></tr>{/each}
    </tbody></table></div>
  {:else}
    <div class="card card-body">
      {#if editing}<WorkFields bind:w section={tab} editing />{:else}<WorkFields w={d.item} section={tab} />{/if}
      {#if tab === 'disposition' && !editing}
        <h3 style="margin-top: 1rem">Disposition approvals</h3>
        <table class="data"><tbody>{#each d.approvals as a}<tr><td>{a.name}</td><td>{#if a.decision}<Status value={a.decision} />{:else}<span class="muted">Pending</span>{/if}</td><td>{dateTime(a.decidedAt)}</td><td>{a.comment}</td></tr>{:else}<tr><td class="empty">No disposition approvers.</td></tr>{/each}</tbody></table>
      {/if}
    </div>
    {#if isNCR && !editing}<p class="small muted">Fields of later stages stay empty until the nonconformance reaches them.</p>{/if}
  {/if}
</div>
{#if d.canSign}
  <div class="decision-bar"><span class="hide-phone">Sign the disposition of {id}</span><span class="spacer"></span><button class="danger" onclick={() => (sign = 'Rejected')}>Reject</button><button class="primary" onclick={() => (sign = 'Approved')}>Approve</button></div>
{/if}
{#if sign}<SignatureDialog title="{sign === 'Approved' ? 'Approve' : 'Reject'} disposition of {id}" decision={sign} requireComment={sign === 'Rejected'} onsign={signNCR} onclose={() => (sign = '')} />{/if}
{#if session.commentsOpen}<Comments recordId={id} />{/if}
{#if lockOpen}<LockDialog recordId={id} current={d.lock} onclose={() => (lockOpen = false)} ondone={() => { lockOpen = false; load(); }} />{/if}
{/if}
