<script lang="ts">
  import type { Snippet } from 'svelte';
  let { label = 'Actions', left = false, children, cls = '' }: { label?: string; left?: boolean; children: Snippet; cls?: string } = $props();
  let open = $state(false);
  let wrap: HTMLElement;
  function outside(e: MouseEvent) {
    if (open && wrap && !wrap.contains(e.target as Node)) open = false;
  }
</script>

<svelte:window onclick={outside} />
<div class="menu-wrap" bind:this={wrap}>
  <button class={cls} aria-haspopup="menu" aria-expanded={open} onclick={() => (open = !open)}>{label} ▾</button>
  {#if open}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="menu" class:left role="menu" onclick={() => (open = false)}>{@render children()}</div>
  {/if}
</div>
