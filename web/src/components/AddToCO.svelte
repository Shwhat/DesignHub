<script lang="ts">
  import { untrack } from 'svelte';
  import Modal from './Modal.svelte';
  import { get, post } from '../lib/api';
  import { toast, session } from '../lib/state.svelte';
  import { navigate } from '../lib/router.svelte';

  let { itemId, status, onclose, ondone }: { itemId: string; status: string; onclose: () => void; ondone: () => void } = $props();
  let cos: any[] = $state([]);
  let choice = $state('new');
  let title = $state('');
  let action = $state(untrack(() => status) === 'Draft' ? 'Release' : 'Revise');
  let busy = $state(false);
  get('/api/change-orders?filter=open').then((r: any[]) => (cos = r.filter((c) => c.state === 'Draft'))).catch(() => {});
  async function go() {
    busy = true;
    try {
      let id = choice;
      if (choice === 'new') {
        const r = await post('/api/change-orders', { title: title || `${action} ${itemId}`, changeType: action === 'Release' ? 'Initial release' : action === 'Obsolete' ? 'Obsolescence' : 'Design change', homeSpace: session.space, items: [{ item: itemId, action }] });
        id = r.id;
      } else {
        await post(`/api/change-orders/${id}/items`, { item: itemId, action });
      }
      toast(`Added to ${id}.`);
      ondone();
      if (choice === 'new') navigate('/change-orders/' + id);
    } catch (e: any) { toast(e.message, 'error'); }
    busy = false;
  }
</script>

<Modal title="Add {itemId} to a change order" {onclose}>
  <div class="field"><label for="ac">Action</label>
    <select id="ac" bind:value={action}>
      {#if status === 'Draft'}<option>Release</option>{:else}<option>Revise</option><option>Obsolete</option>{/if}
    </select></div>
  <div class="field"><label for="co">Change order</label>
    <select id="co" bind:value={choice}><option value="new">New change order</option>{#each cos as c}<option value={c.id}>{c.id} {c.title}</option>{/each}</select></div>
  {#if choice === 'new'}<div class="field"><label for="ct">Title</label><input id="ct" bind:value={title} placeholder="{action} {itemId}" /></div>{/if}
  {#snippet footer()}<button onclick={onclose}>Cancel</button><button class="primary" disabled={busy} onclick={go}>Add</button>{/snippet}
</Modal>
