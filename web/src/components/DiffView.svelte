<script lang="ts">
  export interface DiffLine { op: string; text: string }
  let { lines, mode = 'single' }: { lines: DiffLine[]; mode?: 'single' | 'side' } = $props();
  const left = $derived(lines.filter((l) => l.op !== '+'));
  const right = $derived(lines.filter((l) => l.op !== '-'));
</script>

{#if lines.length === 0}
  <div class="empty">No differences.</div>
{:else if mode === 'side'}
  <div class="diff-cols">
    <div class="diff" aria-label="Before">{#each left as l}<div class:del={l.op === '-'}>{l.text}</div>{/each}</div>
    <div class="diff" aria-label="After">{#each right as l}<div class:add={l.op === '+'}>{l.text}</div>{/each}</div>
  </div>
{:else}
  <div class="diff">
    {#each lines as l}<div class:add={l.op === '+'} class:del={l.op === '-'}>{l.op === '=' ? '  ' : l.op + ' '}{l.text}</div>{/each}
  </div>
{/if}
