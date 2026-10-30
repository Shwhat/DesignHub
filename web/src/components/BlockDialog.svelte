<script lang="ts">
  import Modal from './Modal.svelte';
  import RecordPicker from './RecordPicker.svelte';
  import { blockMarkdown, blockLabels, LIVE_BLOCKS } from '../lib/markdown';
  import { session } from '../lib/state.svelte';

  let { oninsert, onclose }: { oninsert: (md: string) => void; onclose: () => void } = $props();
  let kind = $state('bom');
  let id = $state('');
  let workType = $state('ncr');
  let workSpace = $state('');
  let open = $state(true);

  function insert() {
    let args = '';
    if (kind === 'bom' || kind === 'item' || kind === 'traceability') args = id;
    if (kind === 'work') args = [workType ? 'type=' + workType : '', id ? 'item=' + id : '', workSpace ? 'space=' + workSpace : '', open ? '' : 'open=0'].filter(Boolean).join(' ');
    if (kind === 'changeorders') args = workSpace ? 'space=' + workSpace : '';
    oninsert(blockMarkdown(kind, args));
  }
  const needsId = $derived(kind === 'bom' || kind === 'item');
</script>

<Modal title="Insert live block" {onclose}>
  <div class="field">
    <label for="bk">Block</label>
    <select id="bk" bind:value={kind} onchange={() => (id = '')}>
      {#each LIVE_BLOCKS as k}<option value={k}>{blockLabels[k]}</option>{/each}
    </select>
  </div>
  {#if kind !== 'changeorders'}
    <div class="field">
      <label for="bi">{kind === 'bom' ? 'Assembly' : kind === 'work' ? 'Engineering item (optional)' : kind === 'traceability' ? 'Assembly or requirement (optional)' : 'Record'}</label>
      {#if id}<div class="row"><span class="id">{id}</span><button class="small ghost" onclick={() => (id = '')}>Change</button></div>
      {:else}<RecordPicker kinds={kind === 'bom' ? 'assembly' : kind === 'item' ? '' : 'part,assembly,requirement'} onpick={(v) => (id = v)} />{/if}
    </div>
  {/if}
  {#if kind === 'work'}
    <div class="grid2">
      <div class="field"><label for="wt">Type</label>
        <select id="wt" bind:value={workType}><option value="">Any</option><option value="task">Tasks</option><option value="issue">Issues</option><option value="ncr">Nonconformances</option><option value="action">Actions</option><option value="epic">Epics</option></select></div>
      <div class="field"><label for="ws">Space</label>
        <select id="ws" bind:value={workSpace}><option value="">Any</option>{#each session.me?.spaces || [] as s}<option value={s.id}>{s.name}</option>{/each}</select></div>
    </div>
    <label class="checkbox"><input type="checkbox" bind:checked={open} /> Open items only</label>
  {/if}
  {#if kind === 'changeorders'}
    <div class="field"><label for="cs">Space</label>
      <select id="cs" bind:value={workSpace}><option value="">All</option>{#each session.me?.spaces || [] as s}<option value={s.id}>{s.name}</option>{/each}</select></div>
  {/if}
  {#snippet footer()}
    <button onclick={onclose}>Cancel</button>
    <button class="primary" disabled={needsId && !id} onclick={insert}>Insert</button>
  {/snippet}
</Modal>
