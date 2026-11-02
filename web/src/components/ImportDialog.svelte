<script lang="ts">
  import { untrack } from 'svelte';
  import Modal from './Modal.svelte';
  import Icon from './Icon.svelte';
  import { post, upload } from '../lib/api';
  import { toast } from '../lib/state.svelte';
  import { exportCSV } from '../lib/exporting';

  // type: parts, bom, requirements, work, manufacturers, companies
  let { type, target = '', title, onclose, ondone }: { type: string; target?: string; title: string; onclose: () => void; ondone?: () => void } = $props();

  const isBom = untrack(() => type === 'bom');
  const modes =
    isBom
      ? [
          ['add', 'Add to the Bill of Design'],
          ['replace', 'Replace the Bill of Design'],
        ]
      : [
          ['create', 'Create only'],
          ['update', 'Create and update'],
        ];
  let step = $state(1);
  let file: { sha256: string; name: string } | null = $state(null);
  let mode = $state(isBom ? 'add' : 'create');
  let rows: any[] = $state([]);
  let errors = $state(0);
  let busy = $state(false);
  let result: any = $state(null);
  let input: HTMLInputElement | undefined = $state();

  async function pick(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f) return;
    busy = true;
    try {
      const u = await upload(f);
      file = { sha256: u.sha256, name: f.name };
      step = 2;
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      busy = false;
    }
  }
  const body = () => ({ type, mode, target, sha256: file?.sha256, name: file?.name });
  async function preview() {
    busy = true;
    try {
      const r = await post('/api/imports/preview', body());
      rows = r.rows;
      errors = r.errors;
      step = 3;
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      busy = false;
    }
  }
  async function commit() {
    busy = true;
    try {
      result = await post('/api/imports/commit', body());
      step = 4;
      ondone?.();
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      busy = false;
    }
  }
  function downloadErrors() {
    const cols = Object.keys(rows[0]?.values || {});
    exportCSV(
      title + ' import errors',
      [{ label: 'Row', value: (r: any) => r.row }, ...cols.map((c) => ({ label: c, value: (r: any) => r.values[c] })), { label: 'Error', value: (r: any) => r.error }],
      rows.filter((r) => r.error),
    );
  }
  const counts = $derived({ create: rows.filter((r) => r.outcome === 'Create').length, update: rows.filter((r) => r.outcome === 'Update').length });
  const cols = $derived(Object.keys(rows[0]?.values || {}).slice(0, 4));
</script>

<Modal title="Import {title} (CSV)" wide {onclose}>
  <div class="row small muted" style="margin-bottom: 10px">
    {#each ['File', 'Mode', 'Preview', 'Result'] as s, i}<span style:font-weight={step === i + 1 ? 600 : 400} style:color={step === i + 1 ? 'var(--text)' : ''}>{i + 1}. {s}</span>{#if i < 3}<Icon name="right" size={12} />{/if}{/each}
  </div>
  {#if step === 1}
    <p>Choose a CSV file saved as UTF-8 with a header row. Column names match the export, so an exported file can be changed and imported again.</p>
    <div class="row">
      <button class="primary" disabled={busy} onclick={() => input?.click()}>Choose file...</button>
      <a class="btn" href="/api/import-templates/{type}" download>Download a blank file</a>
    </div>
    <input type="file" accept=".csv,text/csv" bind:this={input} onchange={pick} style="display: none" />
  {:else if step === 2}
    <p><strong>{file?.name}</strong></p>
    {#each modes as [k, l]}<label class="checkbox" style="margin-bottom: 6px"><input type="radio" name="mode" value={k} bind:group={mode} /> {l}</label>{/each}
    <div class="note">
      {#if type === 'bom'}Lines are added to the draft assembly in one save.{:else if type === 'parts' || type === 'requirements'}New items are created as Drafts. Released items are never changed by an import.{:else}Nothing is saved until you confirm the preview.{/if}
    </div>
  {:else if step === 3}
    <div class="row" style="margin-bottom: 8px">
      <span class="status Released">{counts.create} create</span>
      {#if counts.update}<span class="status Open">{counts.update} update</span>{/if}
      {#if errors}<span class="status Rejected">{errors} errors</span><span class="spacer"></span><button class="small" onclick={downloadErrors}>Download errors</button>{/if}
    </div>
    {#if errors}<div class="error-box" style="margin-bottom: 8px">Fix the rows with errors in the file and choose it again. Nothing is imported while any row has an error.</div>{/if}
    <div class="table-wrap" style="max-height: 50vh; overflow: auto; border: 1px solid var(--line); border-radius: 4px">
      <table class="data">
        <thead><tr><th class="num">Row</th><th>Outcome</th><th>Record</th>{#each cols as c}<th>{c}</th>{/each}<th>Error</th></tr></thead>
        <tbody>
          {#each rows as r}
            <tr><td class="num">{r.row}</td><td><span class="status {r.outcome === 'Error' ? 'Rejected' : r.outcome === 'Update' ? 'Open' : 'Released'}">{r.outcome}</span></td>
              <td class="id">{r.id}</td>{#each cols as c}<td>{r.values[c]}</td>{/each}<td style="color: var(--err)">{r.error || ''}</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {:else}
    <p>Import finished: <strong>{result?.created}</strong> created, <strong>{result?.updated}</strong> updated.</p>
    <div class="note">The import and its file are listed in Admin, Imports.</div>
  {/if}
  {#snippet footer()}
    {#if step === 4}
      <button class="primary" onclick={onclose}>Close</button>
    {:else}
      <button onclick={onclose}>Cancel</button>
      {#if step === 2}<button onclick={() => (step = 1)}>Back</button><button class="primary" disabled={busy} onclick={preview}>Preview</button>{/if}
      {#if step === 3}<button onclick={() => (step = 1)}>Choose another file</button><button class="primary" disabled={busy || errors > 0 || !rows.length} onclick={commit}>Import {rows.length} rows</button>{/if}
    {/if}
  {/snippet}
</Modal>
