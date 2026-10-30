<script lang="ts">
  import { get } from '../lib/api';
  import { dateTime } from '../lib/format';
  import Status from '../components/Status.svelte';
  let { params }: { params?: any; pattern?: string } = $props();
  let list: any[] = $state([]);
  let sel = $state('');
  let cmp = $state('');
  let rows: any[] = $state([]);
  get('/api/baselines').then((r) => { list = r; if (r.length) sel = r[0].co; });
  $effect(() => { if (sel) get(`/api/baselines/${sel}${cmp ? '?compare=' + cmp : ''}`).then((r) => (rows = r)); });
</script>

<div class="content">
  <h1>Baselines</h1>
  <p class="muted small">Every release creates a signed baseline. A baseline shows the released data exactly as it was at that release.</p>
  {#if list.length === 0}<div class="card empty">No releases yet.</div>{:else}
  <div class="row" style="margin-bottom: 1rem">
    <label for="b" style="margin: 0">Baseline</label>
    <select id="b" style="width: auto" bind:value={sel}>{#each list as b}<option value={b.co}>{b.co} {b.title} ({dateTime(b.releasedAt)})</option>{/each}</select>
    <label for="c" style="margin: 0">compared with</label>
    <select id="c" style="width: auto" bind:value={cmp}><option value="">(no comparison)</option>{#each list.filter((b) => b.co !== sel) as b}<option value={b.co}>{b.co} {b.title}</option>{/each}</select>
  </div>
  <div class="card table-wrap"><table class="data">
    <thead><tr><th>ID</th><th>Name</th><th>Rev</th><th>Status</th>{#if cmp}<th>Change</th><th>Other rev</th>{/if}</tr></thead>
    <tbody>{#each rows as r}<tr><td><a class="id" href="/items/{encodeURIComponent(r.id)}{r.revision ? '?rev=' + r.revision : ''}">{r.id}</a></td><td>{r.title}</td><td>{r.revision}</td><td><Status value={r.status} /></td>{#if cmp}<td>{r.change}</td><td>{r.otherRevision || ''}</td>{/if}</tr>
    {:else}<tr><td colspan="6" class="empty">{cmp ? 'No differences.' : 'Empty baseline.'}</td></tr>{/each}</tbody>
  </table></div>
  {/if}
</div>
