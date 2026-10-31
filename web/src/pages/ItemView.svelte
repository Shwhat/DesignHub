<script lang="ts">
  import { get, put, post, del, ApiError } from '../lib/api';
  import { session, toast, userName, onEvent, can } from '../lib/state.svelte';
  import { navigate, route, setQuery, setLeaveGuard, recordPath } from '../lib/router.svelte';
  import { dateTime, kindLabel, qty } from '../lib/format';
  import Status from '../components/Status.svelte';
  import Menu from '../components/Menu.svelte';
  import Modal from '../components/Modal.svelte';
  import ItemFields from '../components/ItemFields.svelte';
  import Markdown from '../components/Markdown.svelte';
  import Editor from '../components/Editor.svelte';
  import Files from '../components/Files.svelte';
  import BOMView from '../components/BOMView.svelte';
  import BOMEditor from '../components/BOMEditor.svelte';
  import History from '../components/History.svelte';
  import Comments from '../components/Comments.svelte';
  import LockDialog from '../components/LockDialog.svelte';
  import AddToCO from '../components/AddToCO.svelte';
  import LinksEditor from '../components/LinksEditor.svelte';
  import DiffView from '../components/DiffView.svelte';
  import { onDestroy, untrack } from 'svelte';

  let { params }: { params: any; pattern?: string } = $props();
  const id = untrack(() => params.id);
  let v: any = $state(null);
  let error = $state('');
  let tab = $state(route.query.get('tab') || 'details');
  let editing = $state(false);
  let draft: any = $state(null);
  let base = $state('');
  let comment = $state('');
  let saving = $state(false);
  let lockOpen = $state(false);
  let coOpen = $state(false);
  let conflict: any = $state(null);
  let savedDraft: any = $state(null);
  let backlinks: any[] = $state([]);
  let whereUsed: any = $state(null);
  let dirty = $state(false);

  async function load() {
    try {
      const q = new URLSearchParams();
      if (route.query.get('rev')) q.set('rev', route.query.get('rev')!);
      if (route.query.get('show')) q.set('show', route.query.get('show')!);
      v = await get(`/api/items/${encodeURIComponent(id)}?${q}`);
      error = '';
      backlinks = await get(`/api/records/${encodeURIComponent(id)}/backlinks`);
      if (route.query.get('edit') === '1' && v.perms.edit && !editing) startEdit();
    } catch (e: any) { error = e.status === 404 ? 'not found' : e.message; }
  }
  load();
  const off = onEvent((e) => { if (e.type === 'record' && e.id === id && !editing) load(); });
  onDestroy(() => { off(); setLeaveGuard(null); });

  const isAssembly = $derived(v?.kind === 'assembly');
  const tabs = $derived([
    ['details', 'Details'], ...(isAssembly ? [['bom', 'BOM'], ['where', 'Where-used'], ['validation', 'Validation']] : [['where', 'Where-used']]),
    ['files', 'Files'], ['links', 'Links'], ['revisions', 'Revisions'], ['history', 'History'],
  ] as [string, string][]);
  function setTab(t: string) { tab = t; setQuery({ tab: t === 'details' ? null : t }); if (t === 'where') loadWhere(); }
  async function loadWhere() { whereUsed = await get(`/api/items/${encodeURIComponent(id)}/where-used`); }
  $effect(() => { if (tab === 'where' && !whereUsed) loadWhere(); });

  // The working draft is what Edit changes: the never-released item, the
  // next revision, or the change order copy.
  async function startEdit() {
    if (!v.perms.edit) return;
    const main = v.perms.editTarget === 'main';
    draft = JSON.parse(JSON.stringify(main ? v.current : v.draft));
    base = main ? v.hash : v.draftHash;
    editing = true;
    dirty = false;
    comment = '';
    try {
      const d = await get(`/api/records/${encodeURIComponent(id)}/draft`);
      if (d.draft && d.base === base) savedDraft = d;
    } catch {}
    setLeaveGuard(() => !dirty || confirm('Leave without saving? Your unsaved changes stay in your private draft.'));
  }
  let autosaveTimer: any;
  $effect(() => {
    if (!editing || !draft) return;
    const snapshot = JSON.stringify(draft);
    clearTimeout(autosaveTimer);
    autosaveTimer = setTimeout(() => {
      if (!editing) return;
      dirty = true;
      put(`/api/records/${encodeURIComponent(id)}/draft`, { base, content: JSON.parse(snapshot) }).catch(() => {});
    }, 1200);
  });
  function restoreDraft() { draft = savedDraft.draft; savedDraft = null; }
  async function discardDraft() { savedDraft = null; await del(`/api/records/${encodeURIComponent(id)}/draft`).catch(() => {}); }
  async function cancel() {
    if (dirty && !confirm('Discard your changes?')) return;
    clearTimeout(autosaveTimer);
    editing = false; draft = null; dirty = false;
    setLeaveGuard(null);
    await del(`/api/records/${encodeURIComponent(id)}/draft`).catch(() => {});
    setQuery({ edit: null });
  }
  async function save() {
    saving = true;
    clearTimeout(autosaveTimer);
    try {
      await put(`/api/items/${encodeURIComponent(id)}`, { item: draft, base, comment });
      editing = false; draft = null; dirty = false;
      setLeaveGuard(null);
      setQuery({ edit: null });
      toast('Saved.');
      await load();
    } catch (e: any) {
      if (e instanceof ApiError && e.status === 409) conflict = e.data;
      else toast(e.message, 'error');
    }
    saving = false;
  }
  async function reloadLatest() {
    conflict = null;
    editing = false; dirty = false;
    setLeaveGuard(null);
    await load();
    startEdit();
  }
  async function startRevision() {
    try { await post(`/api/items/${encodeURIComponent(id)}/revision`); toast('New revision started.'); await load(); navigate(`/items/${encodeURIComponent(id)}?show=draft&edit=1`, true); load(); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  async function watch() {
    try { await post(`/api/records/${encodeURIComponent(id)}/watch`, { watch: !v.watching }); v.watching = !v.watching; toast(v.watching ? 'Watching.' : 'Stopped watching.'); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  async function remove() {
    if (!confirm(`Delete draft ${id}? Its number is retired and never used again.`)) return;
    try { await del(`/api/items/${encodeURIComponent(id)}`); toast(`${id} deleted.`); navigate('/library'); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  async function convert() {
    if (!confirm(`Convert reference part ${id} to a production part? A new production number is issued.`)) return;
    try { const r = await post(`/api/items/${encodeURIComponent(id)}/convert`, {}); toast(`Created ${r.id}.`); navigate('/items/' + encodeURIComponent(r.id)); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  const shown = $derived(editing ? draft : v?.item);
  const viewingOld = $derived(v?.showing === 'revision');
  const currentRev = $derived(v?.current?.revision);
</script>

{#if error === 'not found'}
  <div class="content narrow"><div class="card card-body empty"><h2>Not found</h2><p>{id} does not exist, or you do not have access to it.</p></div></div>
{:else if error}
  <div class="content narrow"><div class="error-box">{error}</div></div>
{:else if v}
<div class="content">
  <div class="record-head">
    <div class="titles">
      <div class="meta">
        <span class="id">{id}</span>
        {#if shown.revision}<span>rev {shown.revision}</span>{/if}
        <Status value={v.status} />
        <span class="muted">{kindLabel(v.kind)}</span>
        {#if v.workCount}<a class="pill" href="?tab=links" onclick={(e) => { e.preventDefault(); setTab('links'); }} title="Linked work items">{v.workCount} work item{v.workCount > 1 ? 's' : ''}</a>{/if}
        {#if v.lock.length}<span class="pill" title="Locked to an access list">🔒 {v.lock.length}</span>{/if}
      </div>
      <h1>{shown.title}</h1>
    </div>
    <div class="row">
      <button onclick={() => (session.commentsOpen = !session.commentsOpen)}>Comments</button>
      {#if !editing}
        {#if v.perms.edit && !viewingOld}<button class="primary" onclick={startEdit}>Edit{v.perms.editTarget === 'next' ? ' draft rev ' + v.draft?.revision : v.perms.editTarget === 'co' ? ' on ' + v.co.id : ''}</button>{/if}
        <Menu>
          {#if v.perms.startRevision}<button onclick={startRevision}>Start new revision</button>{/if}
          {#if v.perms.edit && !viewingOld}<button onclick={startEdit}>Edit</button>{/if}
          {#if v.perms.addToCO && can('co.create')}<button onclick={() => (coOpen = true)}>Add to change order</button>{/if}
          <button onclick={watch}>{v.watching ? 'Stop watching' : 'Watch'}</button>
          {#if v.perms.lock}<button onclick={() => (lockOpen = true)}>Lock</button>{/if}
          {#if v.perms.convert}<button onclick={convert}>Convert to production part</button>{/if}
          {#if v.perms.delete}<hr /><button onclick={remove}>Delete draft</button>{/if}
        </Menu>
      {/if}
    </div>
  </div>

  {#if viewingOld}
    <div class="banner warn">You are viewing rev {shown.revision} ({v.status}). Current: <a href="/items/{encodeURIComponent(id)}">rev {currentRev}</a></div>
  {:else if v.draft && v.showing === 'current' && v.current.status !== 'Draft'}
    <div class="banner info">Revision {v.draft.revision} is being drafted{v.co ? ' on ' : ''}{#if v.co}<a href="/change-orders/{v.co.id}">{v.co.id}</a>{/if}. <a href="?show=draft" onclick={(e) => { e.preventDefault(); navigate(`/items/${encodeURIComponent(id)}?show=draft`, true); load(); }}>View draft</a></div>
  {:else if v.showing === 'draft'}
    <div class="banner info">You are viewing the draft of revision {shown.revision}. <a href="/items/{encodeURIComponent(id)}" onclick={(e) => { e.preventDefault(); navigate(`/items/${encodeURIComponent(id)}`, true); load(); }}>View released rev {currentRev}</a></div>
  {/if}
  {#if v.co && !viewingOld}<div class="banner info">On change order <a href="/change-orders/{v.co.id}">{v.co.id} {v.co.title}</a> ({v.co.state}, {v.co.action.toLowerCase()}).</div>{/if}
  {#if v.replacedBy?.length}<div class="banner warn">Replaced by {#each v.replacedBy as r}<a class="id" href="/items/{encodeURIComponent(r)}">{r}</a> {/each}</div>{/if}
  {#if v.perms.editBlocked && !v.perms.edit && !viewingOld && v.perms.startRevision}<div class="muted small" style="margin-bottom: 0.75rem">{v.perms.editBlocked}</div>{/if}

  {#if editing}
    <div class="save-bar">
      <strong>Editing {v.perms.editTarget === 'co' ? 'on ' + v.co.id : v.perms.editTarget === 'next' ? 'draft rev ' + draft.revision : 'draft'}</strong>
      <input placeholder="What changed? (recorded in history)" bind:value={comment} aria-label="Change comment" />
      <button onclick={cancel}>Cancel</button>
      <button class="primary" disabled={saving} onclick={save}>{saving ? 'Saving...' : 'Save'}</button>
    </div>
    {#if savedDraft}<div class="banner info">You have unsaved changes from {dateTime(savedDraft.updatedAt)}. <button class="small" onclick={restoreDraft}>Restore them</button> <button class="small" onclick={discardDraft}>Discard</button></div>{/if}
  {/if}

  <div class="tabs" role="tablist">
    {#each tabs as [k, label]}<button role="tab" class:active={tab === k} onclick={() => setTab(k)}>{label}</button>{/each}
  </div>

  {#if tab === 'details'}
    <div class="card card-body">
      {#if editing}<ItemFields bind:item={draft} editing />{:else}<ItemFields item={shown} />{/if}
      <h3 style="margin-top: 1.25rem">Notes</h3>
      {#if editing}<Editor bind:value={draft.body} recordId={id} />{:else if shown.body}<Markdown source={shown.body} />{:else}<div class="muted small">No notes.</div>{/if}
    </div>
  {:else if tab === 'bom'}
    {#if editing}<BOMEditor bind:lines={draft.bom} parent={id} />
    {:else}<BOMView {id} show={v.showing === 'draft' ? 'draft' : ''} />{/if}
  {:else if tab === 'where'}
    {#if whereUsed}
      <div class="card table-wrap">
        <table class="data"><thead><tr><th>Assembly</th><th>Rev</th><th>Name</th><th>Status</th><th class="num">Qty</th><th>Level</th></tr></thead><tbody>
          {#each whereUsed.usedIn as w}<tr><td><a class="id" href="/items/{encodeURIComponent(w.id)}">{w.id}</a></td><td>{w.revision}</td><td>{w.title}</td><td><Status value={w.status} /></td><td class="num">{w.qty ? qty(w.qty) : ''}</td><td>{w.level}</td></tr>
          {:else}<tr><td colspan="6" class="empty">Not used in any assembly.</td></tr>{/each}
        </tbody></table>
      </div>
      {#if whereUsed.drafts.length}<p class="small muted">Also used in draft revisions of {whereUsed.drafts.join(', ')}.</p>{/if}
    {/if}
  {:else if tab === 'validation'}
    <div class="card">
      {#each v.issues as is}<div class="comment"><span class="issue-dot {is.level}"></span><strong>{is.level === 'error' ? 'Error' : 'Warning'}</strong> {is.message}</div>
      {:else}<div class="empty">No validation issues.</div>{/each}
    </div>
  {:else if tab === 'files'}
    {#if editing}<Files recordId={id} bind:files={draft.files} editing expected={v.expectedFiles} />
    {:else}<Files recordId={id} files={shown.files || []} expected={v.expectedFiles} />{/if}
  {:else if tab === 'links'}
    {#if editing}<LinksEditor bind:links={draft.links} type={draft.type} />{:else}
      <div class="card">
        <div class="card-head"><h3>Links</h3></div>
        {#each shown.links || [] as l}<div class="comment"><span class="muted">{l.type}</span> <a class="id" href={recordPath(l.to)}>{l.to}</a>{#if l.rev} rev {l.rev}{/if}</div>{:else}<div class="empty">No links.</div>{/each}
      </div>
    {/if}
    <div class="card" style="margin-top: 1rem">
      <div class="card-head"><h3>Linked from</h3></div>
      {#each backlinks as b}<div class="comment"><span class="muted">{b.type}</span> <a class="id" href={recordPath(b.src, b.kind)}>{b.src}</a> {b.title} <span class="muted small">{kindLabel(b.kind)}</span></div>{:else}<div class="empty">Nothing links here.</div>{/each}
    </div>
  {:else if tab === 'revisions'}
    <div class="card table-wrap">
      <table class="data"><thead><tr><th>Revision</th><th>Status</th><th>Change order</th><th>Released</th></tr></thead><tbody>
        {#each v.revisions as r}<tr class="clickable" onclick={() => { if (r.status !== 'Draft' && r.status !== 'In Review') { navigate(`/items/${encodeURIComponent(id)}${r.revision === currentRev ? '' : '?rev=' + r.revision}`); load(); } else { navigate(`/items/${encodeURIComponent(id)}?show=draft`); load(); } }}>
          <td>{r.revision}</td><td><Status value={r.status} /></td><td>{#if r.co}<a class="id" href="/change-orders/{r.co}" onclick={(e) => e.stopPropagation()}>{r.co}</a>{/if}</td><td>{dateTime(r.releasedAt)}</td></tr>
        {:else}<tr><td colspan="4" class="empty">Not released yet. A new item has no revision until its first release.</td></tr>{/each}
      </tbody></table>
    </div>
  {:else if tab === 'history'}
    <History recordId={id} />
  {/if}
</div>

{#if session.commentsOpen}<Comments recordId={id} />{/if}
{#if lockOpen}<LockDialog recordId={id} current={v.lock} onclose={() => (lockOpen = false)} ondone={() => { lockOpen = false; load(); }} />{/if}
{#if coOpen}<AddToCO itemId={id} status={v.current.status} onclose={() => (coOpen = false)} ondone={() => { coOpen = false; load(); }} />{/if}
{#if conflict}
  <Modal title="Someone else saved first" wide onclose={() => (conflict = null)}>
    <p>Another person saved {id} after you opened it. Your changes were not saved. These are the differences between their version and yours:</p>
    <DiffView lines={conflict.diff || []} />
    {#snippet footer()}<button onclick={() => (conflict = null)}>Keep editing</button><button class="primary" onclick={reloadLatest}>Load their version</button>{/snippet}
  </Modal>
{/if}
{/if}
