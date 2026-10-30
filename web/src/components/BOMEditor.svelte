<script lang="ts">
  import Sortable from 'sortablejs';
  import RecordPicker from './RecordPicker.svelte';
  import { get } from '../lib/api';
  import { onDestroy } from 'svelte';

  let { lines = $bindable([]), parent }: { lines: any[]; parent: string } = $props();
  let body: HTMLElement | undefined = $state();
  let sortable: Sortable | null = null;
  let titles: Record<string, string> = $state({});
  let refText: string[] = $state((lines || []).map((l) => (l.refdes || []).join(' ')));

  $effect(() => {
    const ids = (lines || []).map((l) => l.item).filter((x) => x && !titles[x]);
    if (ids.length) get('/api/resolve?ids=' + ids.map(encodeURIComponent).join(','), { background: true }).then((r) => {
      const t = { ...titles };
      for (const [k, v] of Object.entries<any>(r)) t[k] = v.title;
      titles = t;
    }).catch(() => {});
  });
  $effect(() => {
    if (!body || sortable) return;
    sortable = Sortable.create(body, {
      handle: '.drag', animation: 120,
      onEnd: (e) => {
        if (e.oldIndex === undefined || e.newIndex === undefined || e.oldIndex === e.newIndex) return;
        const l = [...lines];
        const [m] = l.splice(e.oldIndex, 1);
        l.splice(e.newIndex, 0, m);
        const r = [...refText];
        const [mr] = r.splice(e.oldIndex, 1);
        r.splice(e.newIndex, 0, mr);
        // Find numbers follow the new order in steps of 10.
        lines = l.map((x, i) => ({ ...x, find: (i + 1) * 10 }));
        refText = r;
      },
    });
  });
  onDestroy(() => sortable?.destroy());
  function add(id: string) {
    if (id === parent) return;
    const max = Math.max(0, ...(lines || []).map((l) => l.find || 0));
    lines = [...(lines || []), { find: Math.floor(max / 10) * 10 + 10, item: id, qty: 1, refdes: [], notes: '' }];
    refText = [...refText, ''];
  }
  function setRef(i: number, t: string) {
    refText[i] = t;
    lines[i].refdes = t.split(/[\s,]+/).filter(Boolean);
  }
  function remove(i: number) {
    lines = lines.filter((_, j) => j !== i);
    refText = refText.filter((_, j) => j !== i);
  }
</script>

<div class="card table-wrap">
  <table class="data">
    <thead><tr><th></th><th>Find</th><th>Item</th><th class="num">Qty</th><th>Ref des</th><th class="hide-phone">Notes</th><th></th></tr></thead>
    <tbody bind:this={body}>
      {#each lines || [] as l, i (l.item + ':' + i)}
        <tr>
          <td class="drag" style="cursor: grab" title="Drag to reorder">⠿</td>
          <td><input type="number" style="width: 6ch" bind:value={l.find} aria-label="Find number" /></td>
          <td><span class="id">{l.item}</span> <span class="small muted">{titles[l.item] || ''}</span></td>
          <td class="num"><input type="number" step="any" min="0" style="width: 9ch" bind:value={l.qty} aria-label="Quantity" /></td>
          <td><input style="min-width: 10ch" value={refText[i]} oninput={(e) => setRef(i, (e.target as HTMLInputElement).value)} aria-label="Reference designators" /></td>
          <td class="hide-phone"><input bind:value={l.notes} aria-label="Line notes" /></td>
          <td><button class="small danger" onclick={() => remove(i)}>Remove</button></td>
        </tr>
      {/each}
    </tbody>
  </table>
  {#if !(lines || []).length}<div class="empty">No BOM lines yet.</div>{/if}
</div>
<div class="field" style="margin-top: 0.75rem; max-width: 520px">
  <label for="bomadd">Add a line</label>
  <RecordPicker kinds="part,assembly" placeholder="Type a part ID or name" onpick={(id) => add(id)} />
  <div class="hint">-9XX parts cannot be BOM lines. Validation runs on save and in the Validation tab.</div>
</div>
