<script lang="ts">
  import { lists, loadLists, listOf } from '../lib/lists.svelte';
  import { session, userName, spaceName } from '../lib/state.svelte';
  import UserPicker from './UserPicker.svelte';
  import RecordPicker from './RecordPicker.svelte';
  import Markdown from './Markdown.svelte';

  let { item = $bindable(), editing = false, creating = false }: { item: any; editing?: boolean; creating?: boolean } = $props();
  loadLists();
  const isPart = $derived(item.type === 'part');
  const isCOTS = $derived(item.kind === 'cots');
  let labelsText = $state((item.labels || []).join(', '));
  let toolsText = $state((item.tools || []).join(', '));
  $effect(() => { item.labels = labelsText.split(',').map((s: string) => s.trim()).filter(Boolean); });
  $effect(() => { if (item.type === 'procedure') item.tools = toolsText.split(',').map((s: string) => s.trim()).filter(Boolean); });
  const nameLabel = $derived(isPart ? 'Name' : 'Title');
  const max = $derived(isPart ? 50 : 100);
  function toggleVerification(v: string) {
    const cur: string[] = item.verification || [];
    item.verification = cur.includes(v) ? cur.filter((x) => x !== v) : [...cur, v];
  }
  function addStep() { item.steps = [...(item.steps || []), { instruction: '', acceptance: '' }]; }
  function moveStep(i: number, d: number) { const s = [...item.steps]; const j = i + d; if (j < 0 || j >= s.length) return; [s[i], s[j]] = [s[j], s[i]]; item.steps = s; }
  const spaces = $derived((session.me?.spaces || []).filter((s) => !s.archived));
  const mfrs = $derived(lists.manufacturers.filter((m) => m.active));
</script>

{#snippet view(label: string, value: any)}<dt>{label}</dt><dd>{value || '-'}</dd>{/snippet}

{#if !editing}
  <dl class="props">
    {@render view(nameLabel, item.title)}
    {#if isPart}
      {@render view('Part kind', item.kind === 'cots' ? 'COTS' : item.kind[0].toUpperCase() + item.kind.slice(1))}
      {#if isCOTS}
        {@render view('Manufacturer', lists.manufacturers.find((m) => m.code === item.manufacturer)?.name || item.manufacturer)}
        {@render view('Order number', item.orderNumber)}
        {@render view('Type number', item.typeNumber)}
        {@render view('Lifecycle', item.lifecycle)}
        {@render view('Unit cost', item.unitCost)}
        {@render view('Lead time', item.leadTimeDays ? item.leadTimeDays + ' days' : '')}
        <dt>Approved alternates</dt><dd>{#each item.alternates || [] as a}<a class="id" href="/items/{encodeURIComponent(a)}">{a}</a> {:else}-{/each}</dd>
      {/if}
      {@render view('Unit of measure', item.unit)}
      {@render view('Tracking', item.tracking)}
      {@render view('Commodity code', item.commodity)}
      {@render view('Vendors may download native files', item.vendorDownload ? 'Yes' : 'No')}
      <dt>Description</dt><dd style="white-space: pre-wrap">{item.description || '-'}</dd>
    {:else if item.type === 'requirement'}
      <dt>Statement</dt><dd style="white-space: pre-wrap">{item.statement}</dd>
      <dt>Rationale</dt><dd style="white-space: pre-wrap">{item.rationale || '-'}</dd>
      {@render view('Category', item.category)}
      {@render view('Source', item.source)}
      {@render view('Verification method', (item.verification || []).join(', '))}
      {@render view('Priority', item.priority)}
    {:else if item.type === 'procedure'}
      {@render view('Procedure type', item.procedureType)}
      <dt>Purpose and scope</dt><dd style="white-space: pre-wrap">{item.purpose}</dd>
      {@render view('Tools and equipment', (item.tools || []).join(', '))}
      <dt>Safety and prerequisites</dt><dd style="white-space: pre-wrap">{item.safety || '-'}</dd>
      {@render view('Estimated duration', item.durationMin ? item.durationMin + ' min' : '')}
    {:else if item.type === 'document'}
      {@render view('Document type', item.documentType)}
      <dt>Summary</dt><dd style="white-space: pre-wrap">{item.summary || '-'}</dd>
      {@render view('External reference', item.externalRef)}
      {@render view('Vendors may download native files', item.vendorDownload ? 'Yes' : 'No')}
    {/if}
    {@render view('Owner', userName(item.owner))}
    {@render view('Assignee', userName(item.assignee))}
    {@render view('Created by', userName(item.createdBy))}
    {@render view('Home space', spaceName(item.space))}
    {@render view('Labels', (item.labels || []).join(', '))}
  </dl>
  {#if item.type === 'procedure'}
    <h3 style="margin-top: 1.25rem">Steps</h3>
    <div class="card table-wrap"><table class="data"><thead><tr><th>Step</th><th>Instruction</th><th>Acceptance criteria</th></tr></thead><tbody>
      {#each item.steps || [] as s, i}<tr><td>{i + 1}</td><td><Markdown source={s.instruction} /></td><td>{s.acceptance}</td></tr>{/each}
    </tbody></table></div>
  {/if}
{:else}
  <div class="grid2">
    <div class="field"><label for="f-title">{nameLabel} (at most {max} characters)</label><input id="f-title" maxlength={max} bind:value={item.title} required /></div>
    {#if isPart}
      <div class="field"><label for="f-unit">Unit of measure</label><select id="f-unit" bind:value={item.unit}>{#each listOf('units') as u}<option>{u}</option>{/each}</select></div>
      <div class="field"><label for="f-track">Tracking</label><select id="f-track" bind:value={item.tracking}><option>Lot</option><option>Serial</option><option>None</option></select></div>
      <div class="field"><label for="f-comm">Commodity code</label><select id="f-comm" bind:value={item.commodity}><option value="">None</option>{#each listOf('commodity_codes') as u}<option>{u}</option>{/each}</select></div>
      {#if isCOTS}
        {#if creating}
          <div class="field"><label for="f-mfr">Manufacturer</label><select id="f-mfr" bind:value={item.manufacturer} required><option value="">Choose</option>{#each mfrs as m}<option value={m.code}>{m.code} {m.name}</option>{/each}</select></div>
        {:else}
          <div class="field"><label for="f-mfr2">Manufacturer</label><input id="f-mfr2" value={item.manufacturer} disabled /></div>
          <div class="field"><label for="f-on">Order number</label><input id="f-on" value={item.orderNumber} disabled /></div>
        {/if}
        <div class="field"><label for="f-tn">Type number</label><input id="f-tn" bind:value={item.typeNumber} /></div>
        <div class="field"><label for="f-lc">Lifecycle status</label><select id="f-lc" bind:value={item.lifecycle}><option>Active</option><option>Not Recommended for New Design</option><option>Obsolete</option></select></div>
        <div class="field"><label for="f-uc">Unit cost</label><input id="f-uc" bind:value={item.unitCost} /></div>
        <div class="field"><label for="f-lt">Lead time (days)</label><input id="f-lt" type="number" min="0" bind:value={item.leadTimeDays} /></div>
        <div class="field"><label for="f-alt">Approved alternates</label>
          <div>{#each item.alternates || [] as a}<span class="chip">{a}<button aria-label="Remove" onclick={() => (item.alternates = item.alternates.filter((x: string) => x !== a))}>✕</button></span>{/each}</div>
          <RecordPicker kinds="part" placeholder="Add a COTS part" onpick={(id) => { if (/\./.test(id)) item.alternates = [...(item.alternates || []), id]; }} /></div>
      {/if}
      <label class="checkbox"><input type="checkbox" bind:checked={item.vendorDownload} /> Vendors may download native files</label>
    {:else if item.type === 'requirement'}
      <div class="field"><label for="f-cat">Category</label><select id="f-cat" bind:value={item.category}><option value="">Choose</option>{#each listOf('requirement_categories') as u}<option>{u}</option>{/each}</select></div>
      <div class="field"><label for="f-pri">Priority</label><select id="f-pri" bind:value={item.priority}><option value="">Choose</option><option>Must</option><option>Should</option><option>Could</option></select></div>
      <div class="field"><label for="f-src">Source</label><input id="f-src" bind:value={item.source} /></div>
      <div class="field"><span class="label" style="font-size: 0.85rem; color: var(--text-2); font-weight: 500">Verification method</span><div class="row">{#each ['Inspection', 'Analysis', 'Demonstration', 'Test'] as v}<label class="checkbox"><input type="checkbox" checked={(item.verification || []).includes(v)} onchange={() => toggleVerification(v)} /> {v}</label>{/each}</div></div>
    {:else if item.type === 'procedure'}
      <div class="field"><label for="f-pt">Procedure type</label><select id="f-pt" bind:value={item.procedureType}><option value="">Choose</option>{#each listOf('procedure_types') as u}<option>{u}</option>{/each}</select></div>
      <div class="field"><label for="f-dur">Estimated duration (minutes)</label><input id="f-dur" type="number" min="0" bind:value={item.durationMin} /></div>
      <div class="field"><label for="f-tools">Tools and equipment (comma separated)</label><input id="f-tools" bind:value={toolsText} /></div>
    {:else if item.type === 'document'}
      <div class="field"><label for="f-dt">Document type</label><select id="f-dt" bind:value={item.documentType}><option value="">Choose</option>{#each listOf('document_types') as u}<option>{u}</option>{/each}</select></div>
      <div class="field"><label for="f-er">External reference</label><input id="f-er" bind:value={item.externalRef} /></div>
      <label class="checkbox"><input type="checkbox" bind:checked={item.vendorDownload} /> Vendors may download native files</label>
    {/if}
    <div class="field"><label for="f-owner">Owner</label><UserPicker id="f-owner" byUsername bind:value={item.owner} /></div>
    <div class="field"><label for="f-asg">Assignee</label><UserPicker id="f-asg" byUsername bind:value={item.assignee} placeholder="Nobody" /></div>
    <div class="field"><label for="f-space">Home space</label><select id="f-space" bind:value={item.space}>{#each spaces as s}<option value={s.id}>{s.name}</option>{/each}</select></div>
    <div class="field"><label for="f-labels">Labels (comma separated)</label><input id="f-labels" bind:value={labelsText} /></div>
  </div>
  {#if isPart}<div class="field"><label for="f-desc">Description</label><textarea id="f-desc" bind:value={item.description}></textarea></div>{/if}
  {#if item.type === 'requirement'}
    <div class="field"><label for="f-st">Statement ("shall")</label><textarea id="f-st" bind:value={item.statement} required></textarea></div>
    <div class="field"><label for="f-rat">Rationale</label><textarea id="f-rat" bind:value={item.rationale}></textarea></div>
  {/if}
  {#if item.type === 'procedure'}
    <div class="field"><label for="f-pur">Purpose and scope</label><textarea id="f-pur" bind:value={item.purpose} required></textarea></div>
    <div class="field"><label for="f-saf">Safety and prerequisites</label><textarea id="f-saf" bind:value={item.safety}></textarea></div>
    <h3>Steps</h3>
    {#each item.steps || [] as s, i}
      <div class="card card-body" style="margin-bottom: 0.5rem">
        <div class="row"><strong>Step {i + 1}</strong><span class="spacer"></span>
          <button class="small" aria-label="Move up" onclick={() => moveStep(i, -1)}>↑</button><button class="small" aria-label="Move down" onclick={() => moveStep(i, 1)}>↓</button>
          <button class="small danger" onclick={() => (item.steps = item.steps.filter((_: any, j: number) => j !== i))}>Remove</button></div>
        <div class="field"><label for="st-{i}">Instruction</label><textarea id="st-{i}" bind:value={s.instruction}></textarea></div>
        <div class="field"><label for="sa-{i}">Acceptance criteria</label><input id="sa-{i}" bind:value={s.acceptance} /></div>
      </div>
    {/each}
    <button onclick={addStep}>Add step</button>
  {/if}
  {#if item.type === 'document'}<div class="field"><label for="f-sum">Summary</label><textarea id="f-sum" bind:value={item.summary}></textarea></div>{/if}
{/if}
