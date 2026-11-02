<script lang="ts">
  import { untrack } from 'svelte';
  import { get, patch } from '../lib/api';
  import { toast, setSpace, spaceName } from '../lib/state.svelte';
  import { navigate } from '../lib/router.svelte';
  import { date, hours } from '../lib/format';
  import { exportCSV } from '../lib/exporting';
  import Status from '../components/Status.svelte';
  import Icon from '../components/Icon.svelte';

  let { params }: { params: any; pattern?: string } = $props();
  const id = untrack(() => params.id);
  let d: any = $state(null);
  let error = $state('');
  let editing = $state(false);
  let f = $state({ name: '', goal: '', start: '', end: '' });

  async function load() {
    try { d = await get('/api/sprints/' + id); setSpace(d.sprint.space); } catch (e: any) { error = e.status === 404 ? 'Not found.' : e.message; }
  }
  load();
  async function save() {
    try { await patch('/api/sprints/' + id, f); editing = false; load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  const groups = $derived.by(() => {
    const g: Record<string, any[]> = { open: [], active: [], done: [], cancelled: [] };
    for (const w of d?.items || []) (g[w.category] ||= []).push(w);
    return g;
  });
  const committed = $derived(d?.sprint.committed?.items?.length ?? null);
  const completed = $derived(d?.sprint.completed?.done?.length ?? null);
</script>

{#if error}<div class="content narrow"><div class="error-box">{error}</div></div>
{:else if d}
<div class="content narrow">
  <nav class="small muted" style="margin-bottom: 4px"><a href="/spaces/{d.sprint.space}">{spaceName(d.sprint.space)}</a> / <a href="/spaces/{d.sprint.space}/sprints">Sprints</a></nav>
  <div class="record-head">
    <div class="titles">
      <div class="meta"><span class="id">{d.sprint.id}</span><span class="pill">{d.sprint.state}</span><span>{date(d.sprint.start)} to {date(d.sprint.end)}</span></div>
      <h1>{d.sprint.name}</h1>
      {#if d.sprint.goal}<div class="muted">{d.sprint.goal}</div>{/if}
    </div>
    <div class="row">
      <button class="small" onclick={() => exportCSV(d.sprint.name, [{ label: 'ID', value: (w: any) => w.id }, { label: 'Title', value: (w: any) => w.title }, { label: 'Status', value: (w: any) => w.status }, { label: 'Assignee', value: (w: any) => w.assignee }, { label: 'Estimate', value: (w: any) => w.estimate }, { label: 'Logged', value: (w: any) => w.logged }], d.items)}>Export CSV</button>
      {#if d.canPlan && d.sprint.state !== 'Closed' && !editing}<button onclick={() => { f = { name: d.sprint.name, goal: d.sprint.goal, start: d.sprint.start, end: d.sprint.end }; editing = true; }}>Edit</button>{/if}
      <button class="primary" onclick={() => navigate(`/spaces/${d.sprint.space}/board?sprint=${id}`)}>Open board</button>
    </div>
  </div>
  {#if editing}
    <div class="card card-body" style="margin-bottom: 12px">
      <div class="grid2">
        <div class="field"><label for="e-n">Name</label><input id="e-n" bind:value={f.name} /></div>
        <div class="field"><label for="e-g">Goal</label><input id="e-g" bind:value={f.goal} /></div>
        <div class="field"><label for="e-s">Start</label><input id="e-s" type="date" bind:value={f.start} /></div>
        <div class="field"><label for="e-e">End</label><input id="e-e" type="date" bind:value={f.end} /></div>
      </div>
      <div class="row end"><button onclick={() => (editing = false)}>Cancel</button><button class="primary" onclick={save}>Save</button></div>
    </div>
  {/if}
  <div class="grid3" style="margin-bottom: 12px">
    <div class="card card-body"><div class="muted small">Items</div><div class="big">{d.sprint.done} of {d.sprint.items} done</div><div class="progress"><div style="width: {d.sprint.items ? (d.sprint.done * 100) / d.sprint.items : 0}%"></div></div></div>
    <div class="card card-body"><div class="muted small">Estimate and logged</div><div class="big">{hours(d.sprint.estimate)} / {hours(d.sprint.logged)}</div></div>
    <div class="card card-body"><div class="muted small">Committed and completed</div><div class="big">{committed ?? '-'} / {completed ?? '-'}</div>
      {#if d.sprint.completed?.moved?.length}<div class="small muted">{d.sprint.completed.moved.length} unfinished items moved {d.sprint.completed.movedTo ? 'to the next sprint' : 'to the backlog'}</div>{/if}</div>
  </div>
  {#each [['active', 'In progress'], ['open', 'Not started'], ['done', 'Done'], ['cancelled', 'Cancelled']] as [k, l]}
    {#if groups[k]?.length}
      <h3>{l} <span class="muted small">{groups[k].length}</span></h3>
      <div class="card" style="margin-bottom: 12px">
        {#each groups[k] as w}<div class="comment row"><a class="id" href="/work/{w.id}"><Icon name={w.type} size={14} /> {w.id}</a><span style="flex: 1">{w.title}</span><span class="muted small">{w.assignee}</span><span class="small">{hours(w.estimate)}</span><Status value={w.status} /></div>{/each}
      </div>
    {/if}
  {/each}
  {#if !d.items.length}<div class="card card-body empty">No items in this sprint.</div>{/if}
</div>
{/if}

<style>
  .big { font-size: 16px; font-weight: 600; margin: 2px 0 6px; }
</style>
