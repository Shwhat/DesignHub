<script lang="ts">
  import { get, put } from '../lib/api';
  import { spaceName, onEvent } from '../lib/state.svelte';
  import { date, kindLabel } from '../lib/format';
  import Status from '../components/Status.svelte';
  import { onDestroy } from 'svelte';
  let { params }: { params?: any; pattern?: string } = $props();

  let rows: any[] = $state([]);
  let group = $state<'type' | 'space' | 'none'>('none');
  let order: string[] = $state([]);
  async function load() { rows = await get('/api/my-work'); }
  load();
  get('/api/prefs/mywork', { background: true }).then((p) => { if (p) { group = p.group || 'none'; order = p.order || []; } }).catch(() => {});
  const off = onEvent((e) => { if (e.type === 'board') load(); });
  onDestroy(() => off());
  function save() { put('/api/prefs/mywork', { group, order }).catch(() => {}); }
  const groups = $derived.by(() => {
    if (group === 'none') return [['', rows]] as [string, any[]][];
    const m = new Map<string, any[]>();
    for (const r of rows) { const k = group === 'type' ? r.type : r.space; if (!m.has(k)) m.set(k, []); m.get(k)!.push(r); }
    const keys = [...m.keys()].sort((a, b) => { const ia = order.indexOf(a), ib = order.indexOf(b); return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib) || a.localeCompare(b); });
    return keys.map((k) => [k, m.get(k)!]) as [string, any[]][];
  });
  function moveGroup(k: string, dir: number) {
    const keys = groups.map((g) => g[0]);
    const i = keys.indexOf(k);
    const j = i + dir;
    if (j < 0 || j >= keys.length) return;
    [keys[i], keys[j]] = [keys[j], keys[i]];
    order = keys;
    save();
  }
  const label = (k: string) => (group === 'type' ? kindLabel(k) : spaceName(k));
</script>

<div class="content">
  <div class="row" style="margin-bottom: 1rem">
    <h1 style="margin: 0">My Work</h1>
    <span class="spacer"></span>
    <label for="g" style="margin: 0">Group by</label>
    <select id="g" style="width: auto" bind:value={group} onchange={save}><option value="none">None</option><option value="type">Type</option><option value="space">Space</option></select>
  </div>
  {#each groups as [k, list] (k)}
    {#if k}<div class="row lane-title">{label(k)} <span class="muted small">{list.length}</span><button class="small ghost" aria-label="Move group up" onclick={() => moveGroup(k, -1)}>↑</button><button class="small ghost" aria-label="Move group down" onclick={() => moveGroup(k, 1)}>↓</button></div>{/if}
    <div class="card table-wrap" style="margin-bottom: 1rem">
      <table class="data">
        <thead><tr><th>ID</th><th>Title</th><th>Status</th><th class="hide-phone">Priority</th><th>Due</th><th class="hide-phone">Space</th></tr></thead>
        <tbody>
          {#each list as w (w.id)}
            <tr><td><a class="id" href="/work/{w.id}">{w.id}</a></td><td>{w.title}{#if w.blocked} <span class="status Blocked">blocked</span>{/if}</td><td><Status value={w.status} /></td><td class="hide-phone">{w.priority}</td>
              <td style="white-space: nowrap; {w.overdue ? 'color: var(--danger); font-weight: 600' : ''}">{date(w.dueDate)}{w.overdue ? ' (overdue)' : ''}</td><td class="hide-phone">{spaceName(w.space)}</td></tr>
          {:else}<tr><td colspan="6" class="empty">Nothing assigned to you.</td></tr>{/each}
        </tbody>
      </table>
    </div>
  {/each}
</div>
