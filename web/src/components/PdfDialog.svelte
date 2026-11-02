<script lang="ts">
  import Modal from './Modal.svelte';
  import { exportPDF, paper, setPaper } from '../lib/exporting';
  import { toast } from '../lib/state.svelte';

  // build returns the HTML to print for the chosen options.
  let {
    id,
    options = [],
    build,
    onclose,
  }: {
    id: string;
    options?: { key: string; label: string; on?: boolean }[];
    build: (opts: Record<string, boolean>) => Promise<{ html: string; scope: string; pages?: number }>;
    onclose: () => void;
  } = $props();

  let size = $state(paper());
  let opts: Record<string, boolean> = $state({});
  let busy = $state(false);
  let progress = $state('');

  $effect.pre(() => {
    for (const o of options) if (!(o.key in opts)) opts[o.key] = !!o.on;
  });

  async function run() {
    busy = true;
    progress = 'Preparing the pages...';
    try {
      setPaper(size);
      const r = await build(opts);
      progress = 'Creating the PDF...';
      await exportPDF(id, r.html, r.scope, r.pages || 1, size);
      toast('PDF downloaded');
      onclose();
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      busy = false;
      progress = '';
    }
  }
</script>

<Modal title="Export PDF" {onclose}>
  {#each options as o}
    <label class="checkbox" style="margin-bottom: 6px"><input type="checkbox" bind:checked={opts[o.key]} /> {o.label}</label>
  {/each}
  <div class="field" style="margin-top: 8px">
    <label for="pdf-paper">Paper</label>
    <select id="pdf-paper" bind:value={size} style="width: auto">
      <option value="letter">Letter</option>
      <option value="a4">A4</option>
    </select>
  </div>
  <div class="note">The PDF shows the record as it is on screen, with the record ID, revision and status in the header. Every export is recorded in the audit log.</div>
  {#if progress}<div class="note" style="color: var(--accent)">{progress}</div>{/if}
  {#snippet footer()}
    <button onclick={onclose}>Cancel</button>
    <button class="primary" disabled={busy} onclick={run}>Export</button>
  {/snippet}
</Modal>
