<script lang="ts">
  import { get, post } from '../lib/api';
  import { dateTime } from '../lib/format';
  import DiffView from './DiffView.svelte';
  import { toast } from '../lib/state.svelte';

  let { recordId, canRestore = false, base = '', onrestored }: { recordId: string; canRestore?: boolean; base?: string; onrestored?: () => void } = $props();
  let rows: any[] = $state([]);
  let from = $state('');
  let to = $state('');
  let diff: any = $state(null);
  let side = $state(false);

  async function load() {
    rows = await get(`/api/records/${encodeURIComponent(recordId)}/history`);
    if (rows.length > 1) { to = rows[0].blob; from = rows[1].blob; compare(); } else diff = null;
  }
  $effect(() => { recordId; load(); });
  async function compare() {
    if (!to) return;
    diff = await get(`/api/records/${encodeURIComponent(recordId)}/diff?from=${from}&to=${to}`);
  }
  function show(i: number) {
    to = rows[i].blob;
    from = rows[i + 1]?.blob || '';
    compare();
  }
  async function restore(blob: string) {
    if (!confirm('Restore this version? It is saved as a new version.')) return;
    try {
      await post(`/api/records/${encodeURIComponent(recordId)}/restore`, { blob, base });
      toast('Version restored.');
      onrestored?.();
      load();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  const label = (r: any) => `${dateTime(r.when)} ${r.author}`;
</script>

<div class="stack">
  <div class="card table-wrap">
    <table class="data">
      <thead><tr><th>When</th><th>Who</th><th>Change</th><th class="hide-phone">Where</th><th></th></tr></thead>
      <tbody>
        {#each rows as r, i}
          <tr class:clickable={true} onclick={() => show(i)}>
            <td style="white-space: nowrap">{dateTime(r.when)}</td><td>{r.author}</td><td>{r.message}</td>
            <td class="hide-phone"><span class="pill">{r.file === 'current' ? 'current' : r.file === 'next' ? 'next revision' : 'change order'}</span></td>
            <td>{#if canRestore && i > 0 && !r.deleted}<button class="small" onclick={(e) => { e.stopPropagation(); restore(r.blob); }}>Restore</button>{/if}</td>
          </tr>
        {:else}
          <tr><td colspan="5" class="empty">No history.</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
  {#if rows.length > 1}
    <div class="card card-body">
      <div class="row" style="margin-bottom: 0.6rem">
        <label for="hf" style="margin: 0">Compare</label>
        <select id="hf" style="width: auto" bind:value={from} onchange={compare}><option value="">(empty)</option>{#each rows as r}<option value={r.blob}>{label(r)}</option>{/each}</select>
        <span>with</span>
        <select style="width: auto" bind:value={to} onchange={compare} aria-label="Compare with">{#each rows as r}<option value={r.blob}>{label(r)}</option>{/each}</select>
        <span class="spacer"></span>
        <button class="small" class:primary={!side} onclick={() => (side = false)}>Single</button>
        <button class="small hide-phone" class:primary={side} onclick={() => (side = true)}>Side by side</button>
      </div>
      {#if diff}<DiffView lines={diff.lines} mode={side ? 'side' : 'single'} />{/if}
    </div>
  {/if}
</div>
