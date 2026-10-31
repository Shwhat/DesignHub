<script lang="ts">
  import { untrack } from 'svelte';
  import Modal from './Modal.svelte';
  import UserPicker from './UserPicker.svelte';
  import { put } from '../lib/api';
  import { session, toast } from '../lib/state.svelte';
  let { recordId, current, onclose, ondone }: { recordId: string; current: { id: string }[]; onclose: () => void; ondone: () => void } = $props();
  let users: string[] = $state(untrack(() => current).map((u) => u.id));
  async function save(list: string[]) {
    try {
      await put(`/api/records/${encodeURIComponent(recordId)}/lock`, { users: list });
      toast(list.length ? 'Access list saved.' : 'Record unlocked.');
      ondone();
    } catch (e: any) { toast(e.message, 'error'); }
  }
</script>

<Modal title="Lock to an access list" {onclose}>
  <p class="small muted">People not on the list do not see this record anywhere: not in lists, search, links, BOMs or notifications. You stay on the list.</p>
  <UserPicker multiple bind:value={users} placeholder="Add a person" />
  {#snippet footer()}
    {#if current.length}<button class="danger" onclick={() => save([])}>Unlock</button>{/if}
    <span class="spacer"></span>
    <button onclick={onclose}>Cancel</button>
    <button class="primary" disabled={users.length === 0 || (users.length === 1 && users[0] === session.me?.id && current.length === 0)} onclick={() => save(users)}>Save</button>
  {/snippet}
</Modal>
