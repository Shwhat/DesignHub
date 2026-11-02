<script lang="ts">
  import { put, get, post, upload } from '../lib/api';
  import { session, toast, isVendor } from '../lib/state.svelte';
  import { date, dateTime } from '../lib/format';
  import Markdown from './Markdown.svelte';

  // item is the NCR; canRequest lets internal users request a response.
  let { item, canRequest, onchange }: { item: any; canRequest: boolean; onchange?: () => void } = $props();
  const F = $derived(item.fields || {});
  const mine = $derived(isVendor() && session.me?.company === F.supplier);
  let editing = $state(false);
  let rc = $state('');
  let ca = $state('');
  let req = $state({ requested: '', due: '' });
  let docs: any[] = $state([]);
  let input: HTMLInputElement | undefined = $state();

  async function loadDocs() {
    try { docs = await get(`/api/records/${item.id}/supplier-docs`); } catch {}
  }
  loadDocs();
  function startEdit() {
    rc = F.supplier_root_cause || '';
    ca = F.supplier_corrective_action || '';
    editing = true;
  }
  async function save() {
    try {
      await put(`/api/work/${item.id}/supplier-response`, { rootCause: rc, correctiveAction: ca });
      editing = false;
      toast('Response saved.');
      onchange?.();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  async function request() {
    try {
      await put(`/api/work/${item.id}/supplier-response`, { requested: req.requested || new Date().toISOString().slice(0, 10), due: req.due });
      toast('Supplier response requested.');
      onchange?.();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  async function addDoc(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f) return;
    try {
      const u = await upload(f);
      await post(`/api/records/${item.id}/supplier-docs`, { sha256: u.sha256, name: f.name, company: F.supplier });
      toast('Document added.');
      loadDocs();
    } catch (err: any) { toast(err.message, 'error'); }
    (e.target as HTMLInputElement).value = '';
  }
</script>

<div class="card card-body">
  <dl class="props">
    <dt>Requested</dt><dd>{F.supplier_response_requested ? date(F.supplier_response_requested) : 'Not requested'}</dd>
    <dt>Due</dt><dd>{F.supplier_response_due ? date(F.supplier_response_due) : '-'}</dd>
    <dt>Responded</dt><dd>{F.supplier_responded_on ? dateTime(F.supplier_responded_on) : '-'}</dd>
  </dl>
  {#if editing}
    <div class="field" style="margin-top: 10px"><label for="s-rc">Root cause</label><textarea id="s-rc" rows="5" bind:value={rc}></textarea></div>
    <div class="field"><label for="s-ca">Corrective action</label><textarea id="s-ca" rows="5" bind:value={ca}></textarea></div>
    <div class="row end"><button onclick={() => (editing = false)}>Cancel</button><button class="primary" onclick={save}>Save response</button></div>
  {:else}
    <h3 style="margin-top: 12px">Root cause</h3>
    {#if F.supplier_root_cause}<Markdown source={F.supplier_root_cause} />{:else}<div class="muted small">No response yet.</div>{/if}
    <h3 style="margin-top: 12px">Corrective action</h3>
    {#if F.supplier_corrective_action}<Markdown source={F.supplier_corrective_action} />{:else}<div class="muted small">No response yet.</div>{/if}
    {#if mine}<div class="row end" style="margin-top: 10px"><button class="primary" onclick={startEdit}>Edit response</button></div>{/if}
  {/if}
</div>

<div class="card" style="margin-top: 12px">
  <div class="card-head">Supplier documents<span class="spacer"></span>{#if mine || canRequest}<button class="small" onclick={() => input?.click()}>Add document...</button>{/if}</div>
  <input type="file" bind:this={input} onchange={addDoc} style="display: none" />
  {#each docs as d}
    <div class="comment row"><a href="/api/records/{item.id}/files/{d.sha256}?name={encodeURIComponent(d.name)}">{d.name}</a><span class="spacer"></span><span class="muted small">{d.company}, {d.uploadedBy}, {dateTime(d.uploadedAt)}</span></div>
  {:else}<div class="empty">No supplier documents.</div>{/each}
</div>

{#if canRequest && !isVendor()}
  <div class="card card-body row" style="margin-top: 12px; align-items: flex-end">
    <div style="width: 160px"><label for="s-req">Request date</label><input id="s-req" type="date" bind:value={req.requested} /></div>
    <div style="width: 160px"><label for="s-due">Response due</label><input id="s-due" type="date" bind:value={req.due} /></div>
    <button class="primary" onclick={request}>Request supplier response</button>
    <span class="note">The supplier's Vendor users are notified.</span>
  </div>
{/if}
