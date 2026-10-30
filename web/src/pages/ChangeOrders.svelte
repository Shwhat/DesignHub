<script lang="ts">
  import { get, post } from '../lib/api';
  import { can, session, spaceName, toast } from '../lib/state.svelte';
  import { navigate } from '../lib/router.svelte';
  import { date, ago } from '../lib/format';
  import Status from '../components/Status.svelte';
  import Modal from '../components/Modal.svelte';
  let { params }: { params?: any; pattern?: string } = $props();
  let filter = $state('open');
  let rows: any[] = $state([]);
  let creating = $state(false);
  let title = $state('');
  let changeType = $state('Design change');
  $effect(() => { filter; get('/api/change-orders?filter=' + filter).then((r) => (rows = r)); });
  async function create() {
    try { const r = await post('/api/change-orders', { title, changeType, homeSpace: session.space }); navigate('/change-orders/' + r.id); }
    catch (e: any) { toast(e.message, 'error'); }
  }
</script>

<div class="content">
  <div class="row" style="margin-bottom: 1rem"><h1 style="margin: 0">Change orders</h1><span class="spacer"></span>
    <a class="btn" href="/baselines">Baselines</a>
    {#if can('co.create')}<button class="primary" onclick={() => (creating = true)}>New change order</button>{/if}</div>
  <div class="tabs">
    {#each [['open', 'Open'], ['awaiting', 'Awaiting my approval'], ['release', 'Ready to release'], ['all', 'All']] as [k, l]}<button class:active={filter === k} onclick={() => (filter = k)}>{l}</button>{/each}
  </div>
  <div class="card table-wrap">
    <table class="data">
      <thead><tr><th>ID</th><th>Title</th><th>State</th><th class="hide-phone">Type</th><th class="hide-phone">Owner</th><th class="hide-phone">Items</th><th class="hide-phone">Awaiting</th><th class="hide-phone">Updated</th></tr></thead>
      <tbody>
        {#each rows as c}<tr class="clickable" onclick={() => navigate('/change-orders/' + c.id)}><td class="id">{c.id}</td><td>{c.title}</td><td><Status value={c.state} /></td><td class="hide-phone">{c.changeType}</td><td class="hide-phone">{c.owner}</td><td class="hide-phone">{c.items}</td><td class="hide-phone small">{(c.awaiting || []).join(', ')}</td><td class="hide-phone">{ago(c.updated)}</td></tr>
        {:else}<tr><td colspan="8" class="empty">No change orders.</td></tr>{/each}
      </tbody>
    </table>
  </div>
</div>
{#if creating}
  <Modal title="New change order" onclose={() => (creating = false)}>
    <div class="field"><label for="t">Title</label><input id="t" bind:value={title} maxlength="100" /></div>
    <div class="field"><label for="ct">Change type</label><select id="ct" bind:value={changeType}>{#each ['Initial release', 'Design change', 'Documentation only', 'Obsolescence'] as t}<option>{t}</option>{/each}</select></div>
    {#snippet footer()}<button onclick={() => (creating = false)}>Cancel</button><button class="primary" disabled={!title.trim()} onclick={create}>Create</button>{/snippet}
  </Modal>
{/if}
