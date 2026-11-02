<script lang="ts">
  import { get, put, post, upload } from '../lib/api';
  import { toast, can, isVendor, session } from '../lib/state.svelte';
  import { dateTime } from '../lib/format';

  // Suppliers of a part or assembly, and supplier documents. Suppliers are
  // a business relationship and change without a change order.
  let { itemId, suppliers, docs, onchange, showDocs = false }: { itemId: string; suppliers: any[]; docs: any[]; onchange: () => void; showDocs?: boolean } = $props();
  let editing = $state(false);
  let all: any[] = $state([]);
  let chosen: string[] = $state([]);
  let input: HTMLInputElement | undefined = $state();
  const canEdit = !isVendor() && (can('companies.manage') || can('library.edit'));

  async function startEdit() {
    all = await get('/api/companies?active=1').catch(() => []);
    chosen = suppliers.map((s) => s.id);
    editing = true;
  }
  async function save() {
    try {
      await put(`/api/items/${encodeURIComponent(itemId)}/suppliers`, { companies: chosen });
      editing = false;
      onchange();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  async function addDoc(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f) return;
    try {
      const u = await upload(f);
      await post(`/api/records/${encodeURIComponent(itemId)}/supplier-docs`, { sha256: u.sha256, name: f.name, company: isVendor() ? session.me?.company : suppliers[0]?.id });
      toast('Document added.');
      onchange();
    } catch (err: any) { toast(err.message, 'error'); }
    (e.target as HTMLInputElement).value = '';
  }
</script>

{#if !showDocs}
  <dl class="props" style="margin-top: 8px">
    <dt>Suppliers</dt>
    <dd>
      {#if editing}
        <select multiple size="5" bind:value={chosen} style="max-width: 360px">{#each all as c}<option value={c.id}>{c.name}{c.supplierStatus ? ' (' + c.supplierStatus + ')' : ''}</option>{/each}</select>
        <div class="row" style="margin-top: 4px"><button class="small" onclick={() => (editing = false)}>Cancel</button><button class="small primary" onclick={save}>Save suppliers</button></div>
      {:else}
        {#each suppliers as s, i}{#if i}, {/if}{#if isVendor()}{s.name}{:else}<a href="/companies/{s.id}">{s.name}</a>{/if}{#if s.supplierStatus && s.supplierStatus !== 'Approved'} <span class="status In-Review">{s.supplierStatus}</span>{/if}{:else}<span class="muted">None</span>{/each}
        {#if canEdit} <button class="small ghost" onclick={startEdit}>Change</button>{/if}
      {/if}
    </dd>
  </dl>
{:else}
  <div class="card" style="margin-top: 12px">
    <div class="card-head">Supplier documents<span class="spacer"></span>{#if (isVendor() || canEdit) && suppliers.length}<button class="small" onclick={() => input?.click()}>Add document...</button>{/if}</div>
    <input type="file" bind:this={input} onchange={addDoc} style="display: none" />
    {#each docs as d}
      <div class="comment row"><a href="/api/records/{encodeURIComponent(itemId)}/files/{d.sha256}?name={encodeURIComponent(d.name)}">{d.name}</a><span class="spacer"></span><span class="muted small">{d.company}, {d.uploadedBy}, {dateTime(d.uploadedAt)}</span></div>
    {:else}<div class="empty">No supplier documents. Supplier documents are kept apart from revision files.</div>{/each}
  </div>
{/if}
