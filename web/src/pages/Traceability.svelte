<script lang="ts">
  import { get } from '../lib/api';
  import Status from '../components/Status.svelte';
  import RecordPicker from '../components/RecordPicker.svelte';
  let { params }: { params?: any; pattern?: string } = $props();
  let rows: any[] = $state([]);
  let item = $state('');
  let cov = $state('');
  $effect(() => { get('/api/traceability' + (item ? '?item=' + encodeURIComponent(item) : '')).then((r) => (rows = r)); });
  const shown = $derived(rows.filter((r) => !cov || r.coverage === cov));
</script>

<div class="content">
  <h1>Traceability</h1>
  <div class="row" style="margin-bottom: 1rem">
    {#if item}<span class="chip">Assembly {item}<button aria-label="Clear" onclick={() => (item = '')}>✕</button></span>{:else}<div style="width: 300px"><RecordPicker kinds="assembly,part" placeholder="Limit to an assembly" onpick={(id) => (item = id)} /></div>{/if}
    <select style="width: auto" bind:value={cov} aria-label="Coverage"><option value="">Any coverage</option><option>Complete</option><option>No test</option><option>No design</option><option>No design and no test</option></select>
  </div>
  <div class="card table-wrap"><table class="data">
    <thead><tr><th>Requirement</th><th>Status</th><th>Satisfied by</th><th>Verified by</th><th>Coverage</th></tr></thead>
    <tbody>{#each shown as t}<tr><td><a class="id" href="/items/{t.id}">{t.id}</a> {t.title}</td><td><Status value={t.status} /></td>
      <td>{#each t.satisfiedBy as s}<div><a class="id" href="/items/{encodeURIComponent(s.id)}">{s.id}</a> <span class="small">{s.title}</span></div>{/each}</td>
      <td>{#each t.verifiedBy as s}<div><a class="id" href="/items/{s.id}">{s.id}</a> <span class="small">{s.title}</span></div>{/each}</td>
      <td><span class="status {t.coverage === 'Complete' ? 'Complete' : 'Obsolete'}">{t.coverage}</span></td></tr>
    {:else}<tr><td colspan="5" class="empty">No requirements.</td></tr>{/each}</tbody>
  </table></div>
</div>
