<script lang="ts">
  import { get } from '../lib/api';
  import { navigate } from '../lib/router.svelte';
  import { date } from '../lib/format';
  import { exportCSV } from '../lib/exporting';
  import Status from '../components/Status.svelte';

  // Nonconformances naming the Vendor's company as supplier.
  let rows: any[] = $state([]);
  let open = $state(true);
  get('/api/work?type=ncr&open=0').then((r) => (rows = r)).catch(() => {});
  const shown = $derived(rows.filter((r) => !open || (r.category !== 'done' && r.category !== 'cancelled')));
</script>

<div class="content">
  <div class="row" style="margin-bottom: 10px">
    <h1 style="margin: 0">Nonconformances</h1><span class="spacer"></span>
    <label class="checkbox"><input type="checkbox" bind:checked={open} /> Open only</label>
    <button class="small" onclick={() => exportCSV('Supplier NCRs', [{ label: 'ID', value: (r: any) => r.id }, { label: 'Title', value: (r: any) => r.title }, { label: 'Status', value: (r: any) => r.status }, { label: 'Found on', value: (r: any) => r.foundOn }, { label: 'Severity', value: (r: any) => r.severity }, { label: 'Updated', value: (r: any) => r.updated }], shown)}>Export CSV</button>
  </div>
  <div class="card table-wrap">
    <table class="data">
      <thead><tr><th>ID</th><th>Title</th><th>Found on</th><th>Severity</th><th>Status</th><th>Updated</th></tr></thead>
      <tbody>
        {#each shown as r}<tr class="clickable" onclick={() => navigate('/work/' + r.id)}><td><span class="id">{r.id}</span></td><td>{r.title}</td><td class="id">{r.foundOn}</td><td>{r.severity}</td><td><Status value={r.status} /></td><td>{date(r.updated)}</td></tr>
        {:else}<tr><td colspan="6" class="empty">No nonconformances.</td></tr>{/each}
      </tbody>
    </table>
  </div>
</div>
