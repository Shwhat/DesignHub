<script lang="ts">
  import { session } from '../lib/state.svelte';
  // value holds user ids (or usernames when byUsername is set).
  let { value = $bindable(), multiple = false, byUsername = false, filter, placeholder = 'Choose a person', id }:
    { value: any; multiple?: boolean; byUsername?: boolean; filter?: (u: any) => boolean; placeholder?: string; id?: string } = $props();
  const users = $derived(session.users.filter((u) => u.active && (!filter || filter(u))));
  const key = (u: any) => (byUsername ? u.username : u.id);
  function add(e: Event) {
    const v = (e.target as HTMLSelectElement).value;
    if (v && !value.includes(v)) value = [...value, v];
    (e.target as HTMLSelectElement).value = '';
  }
  const name = (k: string) => session.users.find((u) => key(u) === k)?.displayName || k;
</script>

{#if multiple}
  <div>
    {#each value as v}<span class="chip">{name(v)} <button aria-label="Remove" onclick={() => (value = value.filter((x: string) => x !== v))}>✕</button></span>{/each}
  </div>
  <select {id} onchange={add}>
    <option value="">{placeholder}</option>
    {#each users.filter((u) => !value.includes(key(u))) as u}<option value={key(u)}>{u.displayName} ({u.username})</option>{/each}
  </select>
{:else}
  <select {id} bind:value>
    <option value="">{placeholder}</option>
    {#each users as u}<option value={key(u)}>{u.displayName} ({u.username})</option>{/each}
  </select>
{/if}
