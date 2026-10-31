<script lang="ts">
  import { upload, fileURL, get } from '../lib/api';
  import { bytes } from '../lib/format';
  import { toast } from '../lib/state.svelte';

  let { recordId, files = $bindable([]), editing = false, expected = [] }:
    { recordId: string; files: any[]; editing?: boolean; expected?: string[] } = $props();
  let roles: string[] = $state([]);
  let input: HTMLInputElement | undefined = $state();
  let busy = $state(false);
  let dragging = $state(false);
  $effect(() => {
    get('/api/lists', { background: true }).then((ls: any[]) => {
      roles = (ls.find((l) => l.key === 'file_roles')?.values || []).filter((v: any) => v.active).map((v: any) => v.value);
    }).catch(() => {});
  });
  async function add(list: FileList | File[]) {
    busy = true;
    try {
      for (const f of Array.from(list)) {
        const up = await upload(f);
        files = [...(files || []), { ...up, role: guessRole(f.name) }];
      }
    } catch (e: any) { toast(e.message, 'error'); }
    busy = false;
  }
  function guessRole(name: string) {
    const missing = expected.find((r) => !(files || []).some((f) => f.role === r));
    if (/\.pdf$/i.test(name) && missing) return missing;
    if (/\.(png|jpe?g|gif|webp)$/i.test(name)) return 'Image';
    return missing || 'Other';
  }
  const missingRoles = $derived(expected.filter((r) => !(files || []).some((f) => f.role === r)));
  const previewable = (f: any) => f.mime === 'application/pdf' || (f.mime?.startsWith('image/') && f.mime !== 'image/svg+xml');
</script>

{#if missingRoles.length}<div class="warn-box" style="margin-bottom: 0.75rem">Expected at release: {missingRoles.join(', ')}.</div>{/if}
<div class="card table-wrap">
  <table class="data">
    <thead><tr><th>File</th><th>Role</th><th class="num hide-phone">Size</th><th></th></tr></thead>
    <tbody>
      {#each files || [] as f, i (f.sha256 + i)}
        <tr>
          <td><a href={fileURL(recordId, f)} download>{f.name}</a></td>
          <td>{#if editing}<select bind:value={f.role} style="width: auto">{#each roles as r}<option value={r}>{r}</option>{/each}</select>{:else}{f.role}{/if}</td>
          <td class="num hide-phone">{bytes(f.size)}</td>
          <td style="white-space: nowrap">
            {#if previewable(f)}<a class="btn small" href={fileURL(recordId, f, true)} target="_blank" rel="noopener">Preview</a>{/if}
            {#if editing}<button class="small danger" onclick={() => (files = files.filter((_, j) => j !== i))}>Remove</button>{/if}
          </td>
        </tr>
      {:else}
        <tr><td colspan="4" class="empty">No files.</td></tr>
      {/each}
    </tbody>
  </table>
</div>
{#if editing}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="card card-body" style="margin-top: 0.75rem; text-align: center; border-style: dashed; {dragging ? 'border-color: var(--accent)' : ''}"
    ondragover={(e) => { e.preventDefault(); dragging = true; }} ondragleave={() => (dragging = false)}
    ondrop={(e) => { e.preventDefault(); dragging = false; if (e.dataTransfer?.files.length) add(e.dataTransfer.files); }}>
    <button disabled={busy} onclick={() => input?.click()}>{busy ? 'Uploading...' : 'Add files'}</button>
    <span class="muted small"> or drop files here</span>
    <input type="file" multiple hidden bind:this={input} onchange={(e) => { const t = e.target as HTMLInputElement; if (t.files) add(t.files); t.value = ''; }} />
  </div>
{/if}
