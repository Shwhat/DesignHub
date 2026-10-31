<script lang="ts">
  import { get, put, post, del } from '../lib/api';
  import { canIn, toast, onEvent, setSpace, spaceName, session } from '../lib/state.svelte';
  import { navigate, route } from '../lib/router.svelte';
  import { date, kindLabel } from '../lib/format';
  import Status from '../components/Status.svelte';
  import Modal from '../components/Modal.svelte';
  import { onDestroy, untrack } from 'svelte';
  let { params, pattern }: { params: any; pattern?: string } = $props();
  const space = untrack(() => params.id);
  const ncr = untrack(() => pattern?.includes('ncr'));
  setSpace(space);
  const allCols = [
    { key: 'id', label: 'ID' }, { key: 'type', label: 'Type' }, { key: 'title', label: 'Title' }, { key: 'status', label: 'Status' }, { key: 'priority', label: 'Priority' },
    { key: 'assignee', label: 'Assignee' }, { key: 'dueDate', label: 'Due' }, { key: 'epic', label: 'Epic' }, { key: 'labels', label: 'Labels' },
    { key: 'foundOn', label: 'Found on' }, { key: 'severity', label: 'Severity' }, { key: 'updated', label: 'Updated' },
  ];
  const defCols = ncr ? ['id', 'title', 'status', 'severity', 'foundOn', 'assignee', 'dueDate'] : ['id', 'type', 'title', 'status', 'priority', 'assignee', 'dueDate'];
  let rows: any[] = $state([]);
  let cols: string[] = $state(defCols);
  let f = $state({ text: '', type: route.query.get('type') || '', status: '', assignee: '', open: true });
  let saved: any[] = $state([]);
  let colDialog = $state(false);
  let sortKey = $state('id');
  let dir = $state(1);
  const prefKey = 'worklist.' + (ncr ? 'ncr' : 'work');
  async function load() { rows = await get(`/api/spaces/${space}/work?type=${ncr ? 'ncr' : 'task,issue,action,epic'}`); }
  load();
  get('/api/prefs/' + prefKey, { background: true }).then((p) => { if (Array.isArray(p) && p.length) cols = p; }).catch(() => {});
  get('/api/filters?view=' + prefKey, { background: true }).then((s) => (saved = s)).catch(() => {});
  const off = onEvent((e) => { if (e.type === 'board' && e.space === space) load(); });
  onDestroy(() => off());
  const shown = $derived.by(() => {
    const t = f.text.toLowerCase();
    return rows.filter((r) => (!f.type || r.type === f.type) && (!f.status || r.status === f.status) && (!f.assignee || r.assigneeId === f.assignee)
      && (!f.open || !['Done', 'Cancelled', 'Closed'].includes(r.status)) && (!t || r.id.toLowerCase().includes(t) || r.title.toLowerCase().includes(t)))
      .sort((a, b) => String(a[sortKey] ?? '').localeCompare(String(b[sortKey] ?? ''), undefined, { numeric: true }) * dir);
  });
  function cell(r: any, k: string) {
    if (k === 'type') return kindLabel(r.type);
    if (k === 'dueDate') return date(r.dueDate);
    if (k === 'labels') return (r.labels || []).join(', ');
    if (k === 'updated') return date(r.updated);
    return r[k] ?? '';
  }
  async function saveFilter() {
    const name = prompt('Name for this filter');
    if (!name) return;
    await post('/api/filters', { view: prefKey, name, filter: { ...f } }).catch((e) => toast(e.message, 'error'));
    saved = await get('/api/filters?view=' + prefKey);
  }
  const states = ncr ? ['Reported', 'Dispositioned', 'Corrective Action', 'Verified', 'Closed'] : ['Open', 'In Progress', 'In Review', 'Blocked', 'Done', 'Cancelled'];
  function moveCol(i: number, d: number) { const c = [...cols]; const j = i + d; if (j < 0 || j >= c.length) return; [c[i], c[j]] = [c[j], c[i]]; cols = c; }
</script>

<div class="content">
  <div class="row" style="margin-bottom: 1rem"><h1 style="margin: 0">{ncr ? 'NCR List' : 'List'}</h1><span class="muted">{spaceName(space)}</span><span class="spacer"></span>
    {#if canIn(space, 'work.edit')}<a class="btn primary" href="/work/new?space={space}&type={ncr ? 'ncr' : 'task'}">New {ncr ? 'NCR' : 'item'}</a>{/if}</div>
  <div class="row" style="margin-bottom: 0.75rem">
    <input style="max-width: 240px" placeholder="Filter" bind:value={f.text} aria-label="Filter" />
    {#if !ncr}<select style="width: auto" bind:value={f.type} aria-label="Type"><option value="">All types</option><option value="task">Tasks</option><option value="issue">Issues</option><option value="action">Actions</option><option value="epic">Epics</option></select>{/if}
    <select style="width: auto" bind:value={f.status} aria-label="Status"><option value="">Any status</option>{#each states as s}<option>{s}</option>{/each}</select>
    <select style="width: auto" bind:value={f.assignee} aria-label="Assignee"><option value="">Anyone</option><option value={session.me?.id}>Me</option>{#each session.users.filter((u) => u.active && u.id !== session.me?.id) as u}<option value={u.id}>{u.displayName}</option>{/each}</select>
    <label class="checkbox"><input type="checkbox" bind:checked={f.open} /> Open only</label>
    <span class="spacer"></span>
    {#if saved.length}<select style="width: auto" aria-label="Saved filters" onchange={(e) => { const s = saved.find((x) => x.id === (e.target as HTMLSelectElement).value); if (s) f = { ...f, ...s.filter }; (e.target as HTMLSelectElement).value = ''; }}><option value="">Saved filters</option>{#each saved as s}<option value={s.id}>{s.name}</option>{/each}</select>{/if}
    <button class="small" onclick={saveFilter}>Save filter</button>
    <button class="small hide-phone" onclick={() => (colDialog = true)}>Columns</button>
  </div>
  <div class="card table-wrap">
    <table class="data">
      <thead><tr>{#each cols as c}<th class="sortable" onclick={() => { if (sortKey === c) dir = -dir; else { sortKey = c; dir = 1; } }}>{allCols.find((x) => x.key === c)?.label}{sortKey === c ? (dir > 0 ? ' ▲' : ' ▼') : ''}</th>{/each}</tr></thead>
      <tbody>
        {#each shown as r (r.id)}
          <tr class="clickable" onclick={() => navigate('/work/' + r.id)}>{#each cols as c}<td>{#if c === 'id'}<span class="id">{r.id}</span>{:else if c === 'status'}<Status value={r.status} />{#if r.blocked} <span class="status Blocked">blocked</span>{/if}{:else if c === 'dueDate'}<span style={r.overdue ? 'color: var(--danger); font-weight: 600' : ''}>{cell(r, c)}</span>{:else}{cell(r, c)}{/if}</td>{/each}</tr>
        {:else}<tr><td colspan={cols.length} class="empty">No work items.</td></tr>{/each}
      </tbody>
    </table>
  </div>
</div>
{#if colDialog}
  <Modal title="Columns" onclose={() => (colDialog = false)}>
    {#each cols as c, i}<div class="row" style="margin-bottom: 0.3rem"><span style="flex: 1">{allCols.find((x) => x.key === c)?.label}</span><button class="small" onclick={() => moveCol(i, -1)}>↑</button><button class="small" onclick={() => moveCol(i, 1)}>↓</button><button class="small" disabled={c === 'id'} onclick={() => (cols = cols.filter((x) => x !== c))}>Hide</button></div>{/each}
    <hr />{#each allCols.filter((c) => !cols.includes(c.key)) as c}<button class="small" style="margin: 0 0.3rem 0.3rem 0" onclick={() => (cols = [...cols, c.key])}>Show {c.label}</button>{/each}
    {#snippet footer()}<button onclick={() => (cols = defCols)}>Reset</button><button class="primary" onclick={() => { colDialog = false; put('/api/prefs/' + prefKey, cols).catch(() => {}); }}>Save</button>{/snippet}
  </Modal>
{/if}
