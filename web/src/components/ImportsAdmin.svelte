<script lang="ts">
  import { get } from '../lib/api';
  import { can } from '../lib/state.svelte';
  import { dateTime } from '../lib/format';
  import ImportDialog from './ImportDialog.svelte';

  let rows: any[] = $state([]);
  let samples: any[] = $state([]);
  let mfrImport = $state(false);
  async function load() {
    const r = await get('/api/imports');
    rows = r.imports;
    samples = r.samples;
  }
  load();
  const typeLabel: Record<string, string> = { parts: 'Parts', bom: 'BOM lines', requirements: 'Requirements', work: 'Work items', manufacturers: 'Manufacturers', companies: 'Companies' };
</script>

<div class="row" style="margin-bottom: 8px">
  <span class="muted small">Every CSV import, with the file it used.</span><span class="spacer"></span>
  {#if can('lists.manage')}<button class="small" onclick={() => (mfrImport = true)}>Import manufacturers...</button>{/if}
</div>
<div class="card table-wrap"><table class="data">
  <thead><tr><th>When</th><th>By</th><th>Type</th><th>Target</th><th>Mode</th><th class="num">Rows</th><th class="num">Created</th><th class="num">Updated</th><th>File</th></tr></thead>
  <tbody>
    {#each rows as r}<tr><td style="white-space: nowrap">{dateTime(r.at)}</td><td>{r.by}</td><td>{typeLabel[r.type] || r.type}</td><td class="id">{r.target}</td><td>{r.mode}</td>
      <td class="num">{r.rows}</td><td class="num">{r.created}</td><td class="num">{r.updated}</td><td><a href="/api/imports/{r.id}/file">{r.file}</a></td></tr>
    {:else}<tr><td colspan="9" class="empty">No imports yet.</td></tr>{/each}
  </tbody>
</table></div>
{#if samples.length}
  <h3 style="margin-top: 14px">Sample files</h3>
  <div class="card">{#each samples as s}<div class="comment"><a href="/api/import-samples/{s.sha256}" download={s.name}>{s.name}</a></div>{/each}</div>
{/if}
{#if mfrImport}<ImportDialog type="manufacturers" title="manufacturers" onclose={() => (mfrImport = false)} ondone={load} />{/if}
