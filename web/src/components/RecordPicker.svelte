<script lang="ts">
  import { get } from '../lib/api';
  import { kindLabel } from '../lib/format';
  // Search-as-you-type picker for any record. onpick receives the id.
  let { kinds = '', placeholder = 'Type an ID or title', onpick, autofocus = false }:
    { kinds?: string; placeholder?: string; onpick: (id: string, r: any) => void; autofocus?: boolean } = $props();
  let q = $state('');
  let results: any[] = $state([]);
  let sel = $state(0);
  let timer: any;
  let ctrl: AbortController | null = null;
  function search() {
    clearTimeout(timer);
    timer = setTimeout(async () => {
      if (!q.trim()) { results = []; return; }
      ctrl?.abort();
      ctrl = new AbortController();
      try {
        const r = await get(`/api/search?limit=12&q=${encodeURIComponent(q)}${kinds ? '&kind=' + kinds : ''}`, { signal: ctrl.signal });
        results = r.results;
        sel = 0;
      } catch {}
    }, 150);
  }
  function pick(r: any) {
    onpick(r.id, r);
    q = '';
    results = [];
  }
  function key(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') { sel = Math.min(sel + 1, results.length - 1); e.preventDefault(); }
    if (e.key === 'ArrowUp') { sel = Math.max(sel - 1, 0); e.preventDefault(); }
    if (e.key === 'Enter' && results[sel]) { pick(results[sel]); e.preventDefault(); }
    if (e.key === 'Escape') results = [];
  }
</script>

<div style="position: relative">
  <!-- svelte-ignore a11y_autofocus -->
  <input bind:value={q} oninput={search} onkeydown={key} {placeholder} {autofocus} aria-label={placeholder} autocomplete="off" />
  {#if results.length}
    <div class="picker-results">
      {#each results as r, i}
        <button class:sel={i === sel} onclick={() => pick(r)}><span class="id">{r.id}</span> <span>{r.title}</span> <span class="spacer"></span><span class="muted small">{kindLabel(r.kind)}</span></button>
      {/each}
    </div>
  {/if}
</div>
