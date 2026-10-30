<script lang="ts">
  import { get, post, del } from '../lib/api';
  import { session, toast } from '../lib/state.svelte';
  import { route, setQuery, recordPath, navigate } from '../lib/router.svelte';
  import { kindLabel, kindLabels, ago } from '../lib/format';
  import Status from '../components/Status.svelte';
  let { params }: { params?: any; pattern?: string } = $props();
  const q0 = route.query;
  let f = $state({ q: q0.get('q') || '', space: q0.get('space') || '', kind: q0.get('kind') || '', status: q0.get('status') || '', author: q0.get('author') || '', label: q0.get('label') || '' });
  let results: any[] = $state([]);
  let searched = $state(false);
  let saved: any[] = $state([]);
  let filtersOpen = $state(false);
  async function run() {
    setQuery(f);
    const qs = new URLSearchParams(Object.entries(f).filter(([, v]) => v) as any);
    const r = await get('/api/search?limit=100&' + qs);
    results = r.results;
    searched = true;
  }
  if (Object.values(f).some(Boolean)) run();
  get('/api/filters?view=search', { background: true }).then((s) => (saved = s)).catch(() => {});
  async function save() {
    const name = prompt('Name for this search');
    if (!name) return;
    await post('/api/filters', { view: 'search', name, filter: f }).catch((e) => toast(e.message, 'error'));
    saved = await get('/api/filters?view=search');
  }
  function highlight(s: string) {
    const esc = s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    return esc.replace(/\x01/g, '<mark>').replace(/\x02/g, '</mark>');
  }
  const statuses = ['Draft', 'In Review', 'Released', 'Obsolete', 'Open', 'In Progress', 'Done', 'Blocked', 'Reported', 'Dispositioned', 'Closed', 'Approved', 'Rejected', 'Cancelled'];
</script>

<div class="content">
  <h1>Search</h1>
  <form class="row" style="margin-bottom: 1rem" onsubmit={(e) => { e.preventDefault(); run(); }}>
    <input style="flex: 1; min-width: 200px" placeholder="Search text or an ID" bind:value={f.q} aria-label="Search" />
    <button class="primary">Search</button>
    <button type="button" class="phone-only" onclick={() => (filtersOpen = !filtersOpen)}>Filters</button>
    <button type="button" onclick={save}>Save search</button>
  </form>
  <div class="layout">
    <aside class="card card-body filters" class:open={filtersOpen}>
      <div class="field"><label for="fs">Space</label><select id="fs" bind:value={f.space} onchange={run}><option value="">All</option>{#each session.me?.spaces || [] as s}<option value={s.id}>{s.name}</option>{/each}</select></div>
      <div class="field"><label for="fk">Type</label><select id="fk" bind:value={f.kind} onchange={run}><option value="">All</option>{#each Object.entries(kindLabels) as [k, l]}<option value={k}>{l}</option>{/each}</select></div>
      <div class="field"><label for="fst">Status</label><select id="fst" bind:value={f.status} onchange={run}><option value="">Any</option>{#each statuses as s}<option>{s}</option>{/each}</select></div>
      <div class="field"><label for="fa">Author or owner</label><select id="fa" bind:value={f.author} onchange={run}><option value="">Anyone</option>{#each session.users as u}<option value={u.id}>{u.displayName}</option>{/each}</select></div>
      <div class="field"><label for="fl">Label</label><input id="fl" bind:value={f.label} onchange={run} /></div>
      {#if saved.length}
        <div class="nav-section" style="padding: 0">Saved searches</div>
        {#each saved as s}<div class="row"><button class="ghost small" onclick={() => { f = { ...f, ...s.filter }; run(); }}>{s.name}</button><span class="spacer"></span><button class="ghost small" aria-label="Delete" onclick={async () => { await del('/api/filters/' + s.id); saved = saved.filter((x) => x.id !== s.id); }}>✕</button></div>{/each}
      {/if}
    </aside>
    <div style="min-width: 0">
      {#each results as r}
        <a class="result card" href={recordPath(r.id, r.kind)}>
          <div class="row"><span class="muted small">{kindLabel(r.kind)}</span><span class="id">{r.id}</span>{#if r.revision}<span class="small">rev {r.revision}</span>{/if}<strong>{r.title}</strong><Status value={r.status} /><span class="spacer"></span><span class="muted small">{ago(r.updated)}</span></div>
          {#if r.snippet}<div class="small muted" style="margin-top: 0.25rem">{@html highlight(r.snippet)}</div>{/if}
        </a>
      {:else}{#if searched}<div class="card empty">No results.</div>{/if}{/each}
    </div>
  </div>
</div>

<style>
  .layout { display: grid; grid-template-columns: 240px minmax(0, 1fr); gap: 1rem; align-items: start; }
  .result { display: block; padding: 0.75rem 1rem; margin-bottom: 0.5rem; color: var(--text); }
  .result:hover { text-decoration: none; border-color: var(--border-strong); }
  @media (max-width: 700px) { .layout { grid-template-columns: 1fr; } .filters { display: none; } .filters.open { display: block; } }
</style>
