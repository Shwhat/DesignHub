<script lang="ts">
  import { get, post } from '../lib/api';
  import { session, toast, can } from '../lib/state.svelte';
  import { navigate, route } from '../lib/router.svelte';
  import ItemFields from '../components/ItemFields.svelte';
  let { params }: { params?: any; pattern?: string } = $props();

  const types = [
    { key: 'part', label: 'Part', desc: 'Piece part, purchased part or reference model' },
    { key: 'assembly', label: 'Assembly', desc: 'An item with a Bill of Design (-2XX, -3XX harness, -5XX PCBA)' },
    { key: 'requirement', label: 'Requirement', desc: 'A requirement the design must meet' },
    { key: 'procedure', label: 'Procedure', desc: 'Test, inspection or work procedure' },
    { key: 'document', label: 'Document', desc: 'Drawing, specification or other controlled document' },
  ];
  let type = $state(route.query.get('type') || '');
  let kind = $state(type === 'assembly' ? 'production' : '');
  let offers: Record<string, string> = $state({});
  let base = $state('');
  let suffix = $state('001');
  let refNumber = $state('');
  let orderNumber = $state('');
  let error = $state('');
  let busy = $state(false);
  let item: any = $state(blank());

  function blank() {
    return { type: 'part', kind: '', title: '', owner: session.me?.username || '', assignee: '', space: session.space, labels: [], unit: 'Each', tracking: 'Lot', commodity: '', description: '',
      manufacturer: '', typeNumber: '', lifecycle: 'Active', alternates: [], unitCost: '', leadTimeDays: 0, statement: '', rationale: '', category: '', source: '', verification: [], priority: '',
      procedureType: '', purpose: '', tools: [], safety: '', durationMin: 0, steps: [{ instruction: '', acceptance: '' }], documentType: '', summary: '', externalRef: '', vendorDownload: false, links: [], bom: [], files: [], body: '' };
  }
  async function loadOffers() {
    offers = await get('/api/items/new');
    const [b] = (offers.production || '').split('-');
    base = b;
    suffix = type === 'assembly' ? '201' : '001';
    refNumber = offers.reference;
  }
  if (can('library.edit')) loadOffers();
  function pick(t: string) {
    type = t;
    item = blank();
    item.type = t === 'assembly' ? 'part' : t;
    kind = t === 'assembly' ? 'production' : t === 'part' ? '' : t;
    item.kind = t === 'assembly' ? 'production' : '';
    suffix = t === 'assembly' ? '201' : '001';
  }
  $effect(() => { if (type && !item.type) pick(type); });
  function pickKind(k: string) { kind = k; item.kind = k; }
  const showForm = $derived(type && (item.type !== 'part' || kind));
  const number = $derived(kind === 'production' ? `${base}-${suffix}` : kind === 'reference' ? refNumber : '');
  const nextLabel = $derived(type === 'requirement' ? offers.requirement : type === 'procedure' ? offers.procedure : type === 'document' ? offers.document : '');

  async function create() {
    busy = true; error = '';
    try {
      const payload: any = { item: { ...item }, comment: '' };
      if (kind === 'production' || kind === 'reference') payload.number = number;
      if (kind === 'cots') payload.orderNumber = orderNumber;
      if (item.type === 'part') { item.kind = kind; payload.item.kind = kind; }
      const r = await post('/api/items', payload);
      toast(`${r.id} created.`);
      navigate('/items/' + encodeURIComponent(r.id) + '?edit=1', true);
    } catch (e: any) { error = e.message; }
    busy = false;
  }
</script>

<div class="content narrow">
  <h1>New engineering item</h1>
  {#if !can('library.edit')}<div class="error-box">Your role cannot create engineering items.</div>{:else}
  <div class="card card-body" style="margin-bottom: 1rem">
    <h3>1. Type</h3>
    <div class="grid3">
      {#each types as t}<button class="choice" class:primary={type === t.key} onclick={() => pick(t.key)}><strong>{t.label}</strong><span class="small">{t.desc}</span></button>{/each}
    </div>
  </div>
  {#if type === 'part'}
    <div class="card card-body" style="margin-bottom: 1rem">
      <h3>2. Kind</h3>
      <div class="grid3">
        <button class="choice" class:primary={kind === 'production'} onclick={() => pickKind('production')}><strong>Production</strong><span class="small">Numbered 100042-001</span></button>
        <button class="choice" class:primary={kind === 'reference'} onclick={() => pickKind('reference')}><strong>Reference</strong><span class="small">Concept or keep-out model, 1000042</span></button>
        <button class="choice" class:primary={kind === 'cots'} onclick={() => pickKind('cots')}><strong>COTS</strong><span class="small">Purchased part, PXC.3031212</span></button>
      </div>
    </div>
  {/if}
  {#if showForm}
    <div class="card card-body">
      <h3>{type === 'part' ? '3' : '2'}. Details</h3>
      {#if kind === 'production'}
        <div class="field">
          <label for="nb">Item number</label>
          <div class="row"><input id="nb" style="width: 9ch" inputmode="numeric" maxlength="6" bind:value={base} aria-label="Base number" /><span>-</span><input style="width: 6ch" inputmode="numeric" maxlength="3" bind:value={suffix} aria-label="Suffix" /></div>
          <div class="hint">Next available: {offers.production?.split('-')[0]}. Suffix: 0XX or 1XX piece part, 000 layout, 2XX assembly, 3XX harness, 4XX PCB, 5XX PCBA, 6XX internal electronic component, 9XX interface. The number is reserved when you click Create.</div>
        </div>
      {:else if kind === 'reference'}
        <div class="field"><label for="rn">Reference number</label><input id="rn" style="width: 12ch" bind:value={refNumber} /></div>
      {:else if kind === 'cots'}
        <div class="field"><label for="on">Manufacturer order or catalog number</label><input id="on" bind:value={orderNumber} placeholder="3031212" /><div class="hint">The item number is the manufacturer code, a period and this number, for example PXC.3031212.</div></div>
      {:else if nextLabel}
        <p class="muted small">This item will be numbered {nextLabel} or the next free number.</p>
      {/if}
      <ItemFields bind:item editing creating />
      {#if error}<div class="error-box" style="margin: 1rem 0">{error}</div>{/if}
      <div class="row end" style="margin-top: 1rem"><a class="btn" href="/library">Cancel</a><button class="primary" disabled={busy} onclick={create}>Create</button></div>
    </div>
  {/if}
  {/if}
</div>

<style>
  .choice { flex-direction: column; align-items: flex-start; text-align: left; white-space: normal; min-height: 72px; gap: 0.2rem; }
  .choice.primary .small { color: var(--accent-text); opacity: 0.9; }
</style>
