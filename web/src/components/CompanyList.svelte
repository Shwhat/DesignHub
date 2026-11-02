<script lang="ts">
  import { get, post } from '../lib/api';
  import { can, toast } from '../lib/state.svelte';
  import { navigate } from '../lib/router.svelte';
  import { exportCSV, fullExport } from '../lib/exporting';
  import Modal from './Modal.svelte';
  import Menu from './Menu.svelte';
  import ImportDialog from './ImportDialog.svelte';

  let rows: any[] = $state([]);
  let text = $state('');
  let showInactive = $state(false);
  let creating = $state(false);
  let importing = $state(false);
  let form = $state({ name: '', kinds: ['Vendor'] as string[], supplierStatus: '' });
  const kinds = ['Vendor', 'Contractor', 'Customer', 'Other'];

  async function load() {
    rows = await get('/api/companies');
  }
  load();
  const shown = $derived(
    rows.filter((c) => (showInactive || c.active) && (!text.trim() || (c.id + ' ' + c.name + ' ' + c.kinds.join(' ')).toLowerCase().includes(text.trim().toLowerCase()))),
  );
  async function create() {
    try {
      const r = await post('/api/companies', form);
      creating = false;
      navigate('/companies/' + r.id);
    } catch (e: any) {
      toast(e.message, 'error');
    }
  }
  function exportTable() {
    exportCSV('Companies', [
      { label: 'ID', value: (c: any) => c.id },
      { label: 'Name', value: (c: any) => c.name },
      { label: 'Kind', value: (c: any) => c.kinds },
      { label: 'Supplier status', value: (c: any) => c.supplierStatus },
      { label: 'Supplied items', value: (c: any) => c.supplied },
      { label: 'Active', value: (c: any) => (c.active ? 'Yes' : 'No') },
    ], shown);
  }
</script>

<div class="row" style="margin-bottom: 10px">
  <input style="max-width: 280px" placeholder="Filter by ID, name or kind" bind:value={text} aria-label="Filter" />
  <label class="checkbox"><input type="checkbox" bind:checked={showInactive} /> Include inactive</label>
  <span class="spacer"></span>
  <Menu label="Actions" cls="small">
    <button onclick={exportTable}>Export CSV</button>
    <button onclick={() => fullExport('companies')}>Export all fields (CSV)...</button>
    {#if can('companies.manage') && can('csv.import')}<hr /><button onclick={() => (importing = true)}>Import CSV...</button>{/if}
  </Menu>
  {#if can('companies.manage')}<button class="primary small" onclick={() => (creating = true)}>New company...</button>{/if}
</div>
<div class="card table-wrap">
  <table class="data">
    <thead><tr><th>ID</th><th>Name</th><th>Kind</th><th>Supplier status</th><th class="num">Supplied items</th><th>Active</th></tr></thead>
    <tbody>
      {#each shown as c (c.id)}
        <tr class="clickable" onclick={() => navigate('/companies/' + c.id)}>
          <td><span class="id">{c.id}</span></td><td>{c.name}</td><td>{c.kinds.join(', ')}</td>
          <td>{#if c.supplierStatus}<span class="status {c.supplierStatus === 'Approved' ? 'Approved' : c.supplierStatus === 'Conditional' ? 'In-Review' : 'Rejected'}">{c.supplierStatus}</span>{/if}</td>
          <td class="num">{c.supplied}</td><td>{c.active ? 'Yes' : 'No'}</td>
        </tr>
      {:else}
        <tr><td colspan="6" class="empty">No companies.</td></tr>
      {/each}
    </tbody>
  </table>
</div>
<div class="muted small" style="margin-top: 6px">{shown.length} of {rows.length}</div>

{#if creating}
  <Modal title="New company" onclose={() => (creating = false)}>
    <div class="field"><label for="cn">Name</label><input id="cn" bind:value={form.name} /></div>
    <div class="field"><span class="lbl">Kind</span>
      <div class="row">{#each kinds as k}<label class="checkbox"><input type="checkbox" checked={form.kinds.includes(k)} onchange={(e) => (form.kinds = (e.target as HTMLInputElement).checked ? [...form.kinds, k] : form.kinds.filter((x) => x !== k))} /> {k}</label>{/each}</div>
    </div>
    <div class="field"><label for="cs">Supplier status</label>
      <select id="cs" bind:value={form.supplierStatus}><option value="">Not set</option><option>Approved</option><option>Conditional</option><option>Not approved</option></select>
    </div>
    {#snippet footer()}<button onclick={() => (creating = false)}>Cancel</button><button class="primary" onclick={create}>Create</button>{/snippet}
  </Modal>
{/if}
{#if importing}<ImportDialog type="companies" title="companies" onclose={() => (importing = false)} ondone={load} />{/if}

<style>
  .lbl { font-size: 11.5px; font-weight: 600; color: var(--text-2); display: block; margin-bottom: 3px; }
</style>
