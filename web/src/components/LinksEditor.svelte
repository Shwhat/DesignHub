<script lang="ts">
  import RecordPicker from './RecordPicker.svelte';
  import { recordPath } from '../lib/router.svelte';
  let { links = $bindable([]), type }: { links: any[]; type: string } = $props();
  const types = $derived(
    type === 'part' ? ['satisfies', 'replaced by', 'relates to'] :
    type === 'procedure' ? ['verifies', 'applies to', 'replaced by', 'relates to'] :
    type === 'requirement' ? ['derived from', 'replaced by', 'relates to'] :
    type === 'document' ? ['applies to', 'replaced by', 'relates to'] : ['relates to'],
  );
  let newType = $state('');
  let rev = $state('');
  $effect(() => { if (!newType) newType = types[0]; });
  function add(id: string) {
    if ((links || []).some((l) => l.type === newType && l.to === id)) return;
    links = [...(links || []), { type: newType, to: id, ...(rev ? { rev } : {}) }];
    rev = '';
  }
</script>

<div class="card">
  <div class="card-head"><h3>Links</h3></div>
  {#each links || [] as l, i}
    <div class="comment row"><span class="muted">{l.type}</span> <a class="id" href={recordPath(l.to)}>{l.to}</a>{#if l.rev}<span class="pill">rev {l.rev}</span>{/if}<span class="spacer"></span><button class="small danger" onclick={() => (links = links.filter((_, j) => j !== i))}>Remove</button></div>
  {:else}<div class="empty">No links.</div>{/each}
  <div class="card-body row" style="align-items: flex-end">
    <div style="width: 180px"><label for="lt">Link type</label><select id="lt" bind:value={newType}>{#each types as t}<option>{t}</option>{/each}</select></div>
    <div style="width: 90px"><label for="lr">Pin to rev</label><input id="lr" bind:value={rev} placeholder="any" /></div>
    <div style="flex: 1; min-width: 200px"><label for="lp">Record</label><RecordPicker onpick={(id) => add(id)} /></div>
  </div>
</div>
