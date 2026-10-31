<script lang="ts">
  import { get } from '../lib/api';
  import { qty } from '../lib/format';
  import Status from './Status.svelte';
  import { onEvent } from '../lib/state.svelte';
  import { onDestroy } from 'svelte';

  let { id, show = '', compact = false, startFlat = false }: { id: string; show?: string; compact?: boolean; startFlat?: boolean } = $props();
  let data: any = $state(null);
  let error = $state('');
  let flat = $state(false);
  let collapsed: Record<string, boolean> = $state({});
  const prefKey = $derived('bomtree.' + id);

  async function load() {
    try {
      data = await get(`/api/items/${encodeURIComponent(id)}/bom${show ? '?show=' + show : ''}`);
      error = '';
    } catch (e: any) {
      error = e.message;
    }
  }
  $effect(() => {
    id; show;
    flat = startFlat;
    load();
    if (!compact) {
      get('/api/prefs/' + encodeURIComponent(prefKey), { background: true }).then((v) => { if (v && typeof v === "object" && !Array.isArray(v)) collapsed = v; }).catch(() => {});
    }
  });
  const off = onEvent((e) => { if (e.type === 'record') load(); });
  onDestroy(() => off());

  function toggle(path: string) {
    collapsed = { ...collapsed, [path]: !collapsed[path] };
    if (!compact) fetchPut();
  }
  let t: any;
  function fetchPut() {
    clearTimeout(t);
    t = setTimeout(() => import('../lib/api').then((m) => m.put('/api/prefs/' + encodeURIComponent(prefKey), collapsed).catch(() => {})), 400);
  }
  function rows(node: any, path = '', out: any[] = []) {
    for (const c of node.children || []) {
      const p = path + '/' + c.find + c.id;
      out.push({ ...c, path: p });
      if (c.children?.length && !collapsed[p]) rows(c, p, out);
    }
    return out;
  }
  const issueByFind = $derived(Object.fromEntries((data?.issues || []).filter((i: any) => i.find).map((i: any) => [i.find, i])));
</script>

{#if error}
  <div class="error-box">{error}</div>
{:else if !data}
  <div class="muted small">Loading Bill of Design...</div>
{:else}
  <div class="row" style="margin-bottom: 0.5rem">
    {#if compact}<strong><a href="/items/{encodeURIComponent(id)}">{id}</a> {data.tree.title}</strong>{/if}
    <span class="spacer"></span>
    <div class="row" role="group" aria-label="View">
      <button class="small" class:primary={!flat} onclick={() => (flat = false)}>Tree</button>
      <button class="small" class:primary={flat} onclick={() => (flat = true)}>Flat</button>
    </div>
  </div>
  <div class="table-wrap card">
    {#if !flat}
      <table class="data">
        <thead><tr><th>Find</th><th>ID</th><th class="hide-phone">Rev</th><th>Name</th><th class="hide-phone">Status</th><th class="num">Qty</th><th class="hide-phone">Unit</th><th class="hide-phone">Ref des</th><th class="num">Ext qty</th></tr></thead>
        <tbody>
          {#each rows(data.tree) as r (r.path)}
            <tr class="bom-row">
              <td style="padding-left: {0.75 + (r.level - 1) * 1.1}rem">
                {#if r.children?.length}<button class="twisty" aria-label={collapsed[r.path] ? 'Expand' : 'Collapse'} onclick={() => toggle(r.path)}>{collapsed[r.path] ? '▶' : '▼'}</button>{/if}
                {#if issueByFind[r.find] && r.level === 1}<span class="issue-dot {issueByFind[r.find].level}" title={issueByFind[r.find].message}></span>{/if}
                {r.find}
              </td>
              <td><a class="id" href="/items/{encodeURIComponent(r.id)}">{r.id}</a></td>
              <td class="hide-phone">{r.revision}</td>
              <td>{r.title}{#if r.missing}<span class="muted"> (missing)</span>{/if}{#if r.cycle}<span class="status Obsolete">cycle</span>{/if}</td>
              <td class="hide-phone"><Status value={r.status} /></td>
              <td class="num">{qty(r.qty)}</td>
              <td class="hide-phone">{r.unit}</td>
              <td class="hide-phone small">{(r.refdes || []).join(' ')}</td>
              <td class="num">{qty(r.extended)}</td>
            </tr>
          {:else}
            <tr><td colspan="9" class="empty">No BOM lines.</td></tr>
          {/each}
        </tbody>
      </table>
    {:else}
      <table class="data">
        <thead><tr><th>ID</th><th class="hide-phone">Rev</th><th>Name</th><th class="hide-phone">Status</th><th class="hide-phone">Unit</th><th class="num">Total qty</th></tr></thead>
        <tbody>
          {#each data.flat as f (f.id)}
            <tr><td><a class="id" href="/items/{encodeURIComponent(f.id)}">{f.id}</a></td><td class="hide-phone">{f.revision}</td><td>{f.title}</td><td class="hide-phone"><Status value={f.status} /></td><td class="hide-phone">{f.unit}</td><td class="num">{qty(f.total)}</td></tr>
          {:else}
            <tr><td colspan="6" class="empty">No parts.</td></tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
{/if}
