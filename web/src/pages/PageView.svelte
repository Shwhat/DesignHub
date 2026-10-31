<script lang="ts">
  import { get, put, post, del, ApiError } from '../lib/api';
  import { session, toast, userName, onEvent, can, setSpace } from '../lib/state.svelte';
  import { navigate, route, setQuery, setLeaveGuard } from '../lib/router.svelte';
  import { dateTime } from '../lib/format';
  import { listOf, loadLists } from '../lib/lists.svelte';
  import Status from '../components/Status.svelte';
  import Menu from '../components/Menu.svelte';
  import Modal from '../components/Modal.svelte';
  import Markdown from '../components/Markdown.svelte';
  import Editor from '../components/Editor.svelte';
  import History from '../components/History.svelte';
  import Comments from '../components/Comments.svelte';
  import LockDialog from '../components/LockDialog.svelte';
  import DiffView from '../components/DiffView.svelte';
  import UserPicker from '../components/UserPicker.svelte';
  import { onDestroy, untrack } from 'svelte';

  let { params }: { params: any; pattern?: string } = $props();
  const id = untrack(() => params.id);
  loadLists();
  let v: any = $state(null);
  let error = $state('');
  let editing = $state(false);
  let draft: any = $state(null);
  let base = $state('');
  let comment = $state('');
  let saving = $state(false);
  let dirty = $state(false);
  let showHistory = $state(false);
  let infoOpen = $state(false);
  let lockOpen = $state(false);
  let coOpen = $state(false);
  let coChoice = $state('new');
  let coTitle = $state('');
  let draftCOs: any[] = $state([]);
  let conflict: any = $state(null);
  let savedDraft: any = $state(null);

  async function load() {
    try {
      v = await get(`/api/pages/${id}${route.query.get('show') ? '?show=' + route.query.get('show') : ''}`);
      error = '';
      if (v.item.space) setSpace(v.item.space);
      if (route.query.get('edit') === '1' && v.pagePerms.edit && !editing) startEdit();
    } catch (e: any) { error = e.status === 404 ? 'not found' : e.message; }
  }
  load();
  const off = onEvent((e) => { if (e.type === 'record' && e.id === id && !editing) load(); });
  onDestroy(() => { off(); setLeaveGuard(null); });

  async function startEdit() {
    const main = v.perms.editTarget === 'main';
    draft = JSON.parse(JSON.stringify(main ? v.current : v.draft));
    base = main ? v.hash : v.draftHash;
    editing = true; dirty = false; comment = '';
    try { const d = await get(`/api/records/${id}/draft`); if (d.draft && d.base === base) savedDraft = d; } catch {}
    setLeaveGuard(() => !dirty || confirm('Leave without saving? Your unsaved changes stay in your private draft.'));
  }
  let t: any;
  $effect(() => {
    if (!editing || !draft) return;
    const snap = JSON.stringify(draft);
    clearTimeout(t);
    t = setTimeout(() => { if (!editing) return; dirty = true; put(`/api/records/${id}/draft`, { base, content: JSON.parse(snap) }).catch(() => {}); }, 1200);
  });
  async function cancel() {
    if (dirty && !confirm('Discard your changes?')) return;
    clearTimeout(t);
    editing = false; draft = null; dirty = false; setLeaveGuard(null);
    await del(`/api/records/${id}/draft`).catch(() => {});
    setQuery({ edit: null });
  }
  async function save() {
    saving = true; clearTimeout(t);
    try {
      await put(`/api/pages/${id}`, { item: draft, base, comment });
      editing = false; draft = null; dirty = false; setLeaveGuard(null); setQuery({ edit: null });
      toast('Saved.');
      await load();
    } catch (e: any) {
      if (e instanceof ApiError && e.status === 409) conflict = e.data;
      else toast(e.message, 'error');
    }
    saving = false;
  }
  async function openEditUnderCO() {
    coOpen = true;
    draftCOs = (await get('/api/change-orders?filter=open')).filter((c: any) => c.state === 'Draft');
  }
  async function editUnderCO() {
    try {
      const r = await post(`/api/pages/${id}/edit-under-co`, coChoice === 'new' ? { title: coTitle } : { co: coChoice });
      coOpen = false;
      toast(`Editing under ${r.co}.`);
      await load();
      startEdit();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  async function watch() {
    try { await post(`/api/records/${id}/watch`, { watch: !v.watching }); v.watching = !v.watching; toast(v.watching ? 'Watching.' : 'Stopped watching.'); } catch {}
  }
  async function remove() {
    if (!confirm(`Delete page ${v.item.title}?`)) return;
    try { await del(`/api/pages/${id}`); toast('Page deleted.'); navigate('/spaces/' + v.item.space); } catch (e: any) { toast(e.message, 'error'); }
  }
  const shown = $derived(editing ? draft : v?.item);
</script>

{#if error === 'not found'}
  <div class="content narrow"><div class="card card-body empty"><h2>Not found</h2><p>This page does not exist, or you do not have access to it.</p></div></div>
{:else if error}<div class="content narrow"><div class="error-box">{error}</div></div>
{:else if v}
<div class="content narrow">
  <nav class="small muted" aria-label="Breadcrumbs" style="margin-bottom: 0.4rem">
    {#each v.breadcrumbs as b, i}{#if i > 0}<span> / </span>{/if}<a href="/pages/{b.id}">{b.title}</a>{:else}<a href="/spaces/{v.item.space}">{v.spaceName}</a>{/each}
  </nav>
  <div class="record-head">
    <div class="titles">
      {#if editing}<input style="font-size: 1.4rem; font-weight: 650" maxlength="150" bind:value={draft.title} aria-label="Title" />
      {:else}<h1>{shown.title}</h1>{/if}
      <div class="meta" style="margin-top: 0.3rem">
        {#if v.controlled}<Status value={v.status} />{#if shown.revision}<span>rev {shown.revision}</span>{/if}{/if}
        <button class="ghost small" onclick={() => (infoOpen = !infoOpen)}>{infoOpen ? 'Hide details' : 'Page details'}</button>
      </div>
    </div>
    <div class="row">
      <button onclick={() => (session.commentsOpen = !session.commentsOpen)}>Comments</button>
      {#if !editing}
        {#if v.pagePerms.edit}<button class="primary" onclick={startEdit}>Edit{v.perms.editTarget === 'co' ? ' on ' + v.co.id : ''}</button>
        {:else if v.pagePerms.editUnderCO}<button class="primary" onclick={openEditUnderCO}>Edit under change order</button>{/if}
        <Menu>
          <button onclick={() => (showHistory = !showHistory)}>{showHistory ? 'Hide history' : 'History'}</button>
          <button onclick={watch}>{v.watching ? 'Stop watching' : 'Watch'}</button>
          {#if v.pagePerms.lock}<button onclick={() => (lockOpen = true)}>Lock</button>{/if}
          <button onclick={() => window.print()}>Print</button>
          {#if v.pagePerms.delete}<hr /><button onclick={remove}>Delete page</button>{/if}
        </Menu>
      {/if}
    </div>
  </div>

  {#if infoOpen}
    <div class="card card-body" style="margin-bottom: 1rem">
      {#if editing}
        <div class="grid2">
          <div class="field"><label for="pt">Page type</label><select id="pt" bind:value={draft.pageType}>{#each listOf('page_types') as p}<option>{p}</option>{/each}</select></div>
          <div class="field"><label for="pc">Controlled</label><select id="pc" bind:value={draft.controlled}><option value="default">Use type default</option><option value="yes">Yes</option><option value="no">No</option></select></div>
          <div class="field"><label for="po">Owner</label><UserPicker id="po" byUsername bind:value={draft.owner} /></div>
          {#if v.controlled}<label class="checkbox"><input type="checkbox" bind:checked={draft.releasedOnly} /> Readers see the released revision only</label>{/if}
        </div>
      {:else}
        <dl class="props">
          <dt>ID</dt><dd class="id">{id}</dd>
          <dt>Type</dt><dd>{shown.pageType}</dd>
          <dt>Owner</dt><dd>{userName(shown.owner)}</dd>
          <dt>Controlled</dt><dd>{v.controlled ? 'Yes' : 'No'}{shown.controlled === 'default' ? ' (type default)' : ''}</dd>
          {#if v.controlled}<dt>Revision</dt><dd>{shown.revision || 'Not released'}</dd><dt>Readers see</dt><dd>{v.current.releasedOnly ? 'Released revision only' : 'Latest draft'}</dd>{/if}
        </dl>
      {/if}
    </div>
  {/if}

  {#if v.co && !editing}<div class="banner info">On change order <a href="/change-orders/{v.co.id}">{v.co.id}</a> ({v.co.state}).{#if v.showing === 'draft'} You are viewing the draft. <a href="/pages/{id}?show=current" onclick={(e) => { e.preventDefault(); navigate(`/pages/${id}?show=current`, true); load(); }}>View released</a>{/if}</div>
  {:else if v.showing === 'draft' && !editing}<div class="banner info">You are viewing the latest draft of rev {shown.revision}. <a href="/pages/{id}?show=current" onclick={(e) => { e.preventDefault(); navigate(`/pages/${id}?show=current`, true); load(); }}>View released rev {v.current.revision}</a></div>{/if}

  {#if editing}
    <div class="save-bar">
      <strong>Editing{v.perms.editTarget === 'co' ? ' on ' + v.co.id : ''}</strong>
      <input placeholder="What changed? (recorded in history)" bind:value={comment} aria-label="Change comment" />
      <button onclick={cancel}>Cancel</button>
      <button class="primary" disabled={saving} onclick={save}>{saving ? 'Saving...' : 'Save'}</button>
    </div>
    {#if savedDraft}<div class="banner info">You have unsaved changes from {dateTime(savedDraft.updatedAt)}. <button class="small" onclick={() => { draft = savedDraft.draft; savedDraft = null; }}>Restore them</button> <button class="small" onclick={() => { savedDraft = null; del(`/api/records/${id}/draft`).catch(() => {}); }}>Discard</button></div>{/if}
    <Editor bind:value={draft.body} recordId={id} />
  {:else}
    <article class="card card-body">{#if shown.body}<Markdown source={shown.body} />{:else}<div class="muted">This page is empty.</div>{/if}</article>
  {/if}

  {#if v.children.length && !editing}
    <div class="card" style="margin-top: 1rem"><div class="card-head"><h3>Child pages</h3></div>
      {#each v.children as c}<div class="comment"><a href="/pages/{c.id}">{c.title}</a></div>{/each}</div>
  {/if}
  {#if showHistory}<div style="margin-top: 1rem"><History recordId={id} canRestore={v.pagePerms.edit} base={v.perms.editTarget === 'co' ? v.draftHash : v.hash} onrestored={load} /></div>{/if}
</div>

{#if session.commentsOpen}<Comments recordId={id} canComment={v.pagePerms.comment} />{/if}
{#if lockOpen}<LockDialog recordId={id} current={v.lock} onclose={() => (lockOpen = false)} ondone={() => { lockOpen = false; load(); }} />{/if}
{#if coOpen}
  <Modal title="Edit under change order" onclose={() => (coOpen = false)}>
    <p class="small muted">This page is controlled. Changes are made on a change order and published when it is released.</p>
    <div class="field"><label for="cc">Change order</label><select id="cc" bind:value={coChoice}><option value="new">New change order</option>{#each draftCOs as c}<option value={c.id}>{c.id} {c.title}</option>{/each}</select></div>
    {#if coChoice === 'new'}<div class="field"><label for="ct">Title</label><input id="ct" bind:value={coTitle} placeholder="Update {v.item.title}" /></div>{/if}
    {#snippet footer()}<button onclick={() => (coOpen = false)}>Cancel</button><button class="primary" onclick={editUnderCO}>Continue</button>{/snippet}
  </Modal>
{/if}
{#if conflict}
  <Modal title="Someone else saved first" wide onclose={() => (conflict = null)}>
    <p>Another person saved this page after you opened it. Your changes were not saved. Differences between their version and yours:</p>
    <DiffView lines={conflict.diff || []} />
    {#snippet footer()}<button onclick={() => (conflict = null)}>Keep editing</button><button class="primary" onclick={async () => { conflict = null; editing = false; dirty = false; setLeaveGuard(null); await load(); startEdit(); }}>Load their version</button>{/snippet}
  </Modal>
{/if}
{/if}
