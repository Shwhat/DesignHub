<script lang="ts">
  import type { Snippet } from 'svelte';
  let { title, wide = false, onclose, children, footer }: { title: string; wide?: boolean; onclose: () => void; children: Snippet; footer?: Snippet } = $props();
  function key(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose();
  }
</script>

<svelte:window onkeydown={key} />
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-back" onclick={(e) => e.target === e.currentTarget && onclose()}>
  <div class="modal" class:wide role="dialog" aria-modal="true" aria-label={title}>
    <div class="modal-head">
      <h2>{title}</h2>
      <button class="ghost icon" aria-label="Close" onclick={onclose}>✕</button>
    </div>
    <div class="modal-body">{@render children()}</div>
    {#if footer}<div class="modal-foot">{@render footer()}</div>{/if}
  </div>
</div>
