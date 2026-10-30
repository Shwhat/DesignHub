<script lang="ts">
  import { get, put, post, del } from '../lib/api';
  import { can, userName, spaceName, toast } from '../lib/state.svelte';
  import { navigate, route, setQuery } from '../lib/router.svelte';
  import { date } from '../lib/format';
  import Status from '../components/Status.svelte';
  import Modal from '../components/Modal.svelte';
  let { params }: { params: any; pattern?: string } = $props();

  const tabs = [
    { key: 'part', label: 'Parts' },
    { key: 'assembly', label: 'Assemblies' },
    { key: 'requirement', label: 'Requirements' },
    { key: 'procedure', label: 'Procedures' },
    { key: 'document', label: 'Documents' },
  ];
  const allColumns = [
    { key: 'id', label: 'ID' },
    { key: 'revision', label: 'Rev' },
    { key: 'title', label: 'Name' },
    { key: 'status', label: 'Status' },
    { key: 'owner', label: 'Owner' },
    { key: 'space', label: 'Home space' },
    { key: 'partKind', label: 'Part kind' },
    { key: 'extra', label: 'Type' },
    { key: 'labels', label: 'Labels' },
    { key: 'updated', label: 'Updated' },
  ];
  const defaults: Record<string, string[]> = {
    part: ['id', 'revision', 'title', 'status', 'owner', 'space', 'partKind'],
    assembly: ['id', 'revision', 'title', 'status', 'owner', 'space'],
    requirement: ['id', 'revision', 'title', 'status', 'owner', 'extra'],
    procedure: ['id', 'revision', 'title', 'status', 'owner', 'extra'],
    document: ['id', 'revision', 'title', 'status', 'owner', 'extra'],
  };
  let tab = $state(params.tab || 'part');
  let rows: any[] = $state([]);
  let all = $state(route.query.get('all') === '1');
  let text = $state(route.query.get('q') || '');
  let status = $state(route.query.get('status') || '');
  let cols: string[] = $state(defaults[params.tab || 'part'] || defaults.part);
  let colDialog = $state(false);
  let simple = $state(false);
  let saved: any[] = $state([]);
  let sortKey = $state('id');
  let sortDir = $state(1);

  async function load() {
    rows = await get(`/api/library?kind=${tab}${all ? '&all=1' : ''}`);
  }
  async function loadPrefs() {
    try { const c = await get('/api/prefs/library.cols.' + tab, { background: true }); cols = c || defaults[tab]; } catch { cols = defaults[tab]; }
    try { saved = await get('/api/filters?view=library', { background: true }); } catch {}
  }
  $effect(() => { tab; all; load(); });
  $effect(() => { tab; loadPrefs(); });

  const shown = $derived.by(() => {
    const t = text.trim().toLowerCase();
    let r = rows.filter((x) => (!status || x.status === status) && (!t || x.id.toLowerCase().includes(t) || x.title.toLowerCase().includes(t) || (x.labels || []).some((l: string) => l.toLowerCase().includes(t))));
    return [...r].sort((a, b) => String(a[sortKey] ?? '').localeCompare(String(b[sortKey] ?? ''), undefined, { numeric: true }) * sortDir);
  });
  function sortBy(k: string) { if (sortKey === k) sortDir = -sortDir; else { sortKey = k; sortDir = 1; } }
  function cell(r: any, k: string) {
    switch (k) {
      case 'owner': return userName(r.owner);
      case 'space': return spaceName(r.space);
      case 'partKind': return r.partKind === 'cots' ? 'COTS' : r.partKind ? r.partKind[0].toUpperCase() + r.partKind.slice(1) : '';
      case 'labels': return (r.labels || []).join(', ');
      case 'updated': return date(r.updated);
      default: return r[k] ?? '';
    }
  }
  function setTab(t: string) { tab = t; navigate('/library/' + t); }
  async function saveCols() {
    colDialog = false;
    put('/api/prefs/library.cols.' + tab, cols).catch(() => {});
  }
  async function saveFilter() {
    const name = prompt('Name for this filter');
    if (!name) return;
    try { await post('/api/filters', { view: 'library', name, filter: { tab, text, status, all } }); saved = await get('/api/filters?view=library'); toast('Filter saved.'); } catch (e: any) { toast(e.message, 'error'); }
  }
  function applyFilter(f: any) {
    const v = f.filter;
    if (v.tab !== tab) setTab(v.tab);
    text = v.text || ''; status = v.status || ''; all = !!v.all;
  }
  async function deleteFilter(f: any) { await del('/api/filters/' + f.id).catch(() => {}); saved = saved.filter((x) => x.id !== f.id); }
  function moveCol(i: number, d: number) { const c = [...cols]; const j = i + d; if (j < 0 || j >= c.length) return; [c[i], c[j]] = [c[j], c[i]]; cols = c; }
  $effect(() => { setQuery({ q: text || null, status: status || null, all: all ? '1' : null }); });
</script>

<div class="content">
  <div class="row" style="margin-bottom: 0.75rem">
    <h1 style="margin: 0">Library</h1>
    <span class="spacer"></span>
    {#if can('library.edit')}<a class="btn primary" href="/items/new?type={tab === 'assembly' ? 'assembly' : tab}">New {tabs.find((t) => t.key === tab)?.label.slice(0, -1).toLowerCase()}</a>{/if}
  </div>
  <div class="tabs" role="tablist">
    {#each tabs as t}<button role="tab" class:active={tab === t.key} aria-selected={tab === t.key} onclick={() => setTab(t.key)}>{t.label}</button>{/each}
  </div>
  <div class="row" style="margin-bottom: 0.75rem">
    <input style="max-width: 280px" placeholder="Filter by ID, name or label" bind:value={text} aria-label="Filter" />
    <select style="width: auto" bind:value={status} aria-label="Status">
      <option value="">Any status</option>{#each ['Draft', 'In Review', 'Released', 'Superseded', 'Obsolete'] as s}<option>{s}</option>{/each}
    </select>
    <label class="checkbox"><input type="checkbox" bind:checked={all} /> Include superseded and obsolete</label>
    <span class="spacer"></span>
    {#if saved.length}
      <select style="width: auto" aria-label="Saved filters" onchange={(e) => { const f = saved.find((x) => x.id === (e.target as HTMLSelectElement).value); if (f) applyFilter(f); (e.target as HTMLSelectElement).value = ''; }}>
        <option value="">Saved filters</option>{#each saved as f}<option value={f.id}>{f.name}</option>{/each}
      </select>
    {/if}
    <button class="small" onclick={saveFilter}>Save filter</button>
    <button class="small hide-phone" onclick={() => (colDialog = true)}>Columns</button>
    <button class="small phone-only" onclick={() => (simple = !simple)}>{simple ? 'Table' : 'Simple list'}</button>
  </div>
  {#if simple}
    <div class="card">
      {#each shown as r (r.id + r.revision)}<a class="sline" href="/items/{encodeURIComponent(r.id)}{r.status === 'Superseded' ? '?rev=' + r.revision : ''}"><span class="id">{r.id}</span><span style="flex: 1">{r.title}</span><Status value={r.status} /></a>{:else}<div class="empty">No items.</div>{/each}
    </div>
  {:else}
    <div class="card table-wrap">
      <table class="data">
        <thead><tr>{#each cols as c}<th class="sortable" onclick={() => sortBy(c)}>{allColumns.find((x) => x.key === c)?.label}{sortKey === c ? (sortDir > 0 ? ' ▲' : ' ▼') : ''}</th>{/each}</tr></thead>
        <tbody>
          {#each shown as r (r.id + r.revision + r.status)}
            <tr class="clickable" onclick={() => navigate('/items/' + encodeURIComponent(r.id) + (r.status === 'Superseded' ? '?rev=' + r.revision : ''))}>
              {#each cols as c}
                <td>
                  {#if c === 'id'}<span class="id">{r.id}</span>{#if r.locked} <span title="Locked">🔒</span>{/if}
                  {:else if c === 'status'}<Status value={r.status} />{#if r.draft} <span class="pill">rev {r.draft} draft</span>{/if}
                  {:else}{cell(r, c)}{/if}
                </td>
              {/each}
            </tr>
          {:else}
            <tr><td colspan={cols.length} class="empty">No items.</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
  <div class="muted small" style="margin-top: 0.5rem">{shown.length} of {rows.length}</div>
</div>

{#if colDialog}
  <Modal title="Columns" onclose={() => (colDialog = false)}>
    {#each cols as c, i}
      <div class="row" style="margin-bottom: 0.3rem"><span style="flex: 1">{allColumns.find((x) => x.key === c)?.label}</span>
        <button class="small" onclick={() => moveCol(i, -1)} aria-label="Up">↑</button><button class="small" onclick={() => moveCol(i, 1)} aria-label="Down">↓</button>
        <button class="small" disabled={c === 'id'} onclick={() => (cols = cols.filter((x) => x !== c))}>Hide</button></div>
    {/each}
    <hr />
    {#each allColumns.filter((c) => !cols.includes(c.key)) as c}<button class="small" style="margin: 0 0.3rem 0.3rem 0" onclick={() => (cols = [...cols, c.key])}>Show {c.label}</button>{/each}
    {#snippet footer()}<button onclick={() => (cols = defaults[tab])}>Reset</button><button class="primary" onclick={saveCols}>Save</button>{/snippet}
  </Modal>
{/if}

<style>
  .sline { display: flex; gap: 0.6rem; align-items: center; padding: 0.6rem 0.9rem; border-bottom: 1px solid var(--border); color: var(--text); }
  .sline:hover { text-decoration: none; background: var(--surface-2); }
</style>
