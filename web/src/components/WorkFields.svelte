<script lang="ts">
  import { listOf, loadLists, lists } from '../lib/lists.svelte';
  import { userName, session } from '../lib/state.svelte';
  import { date } from '../lib/format';
  import UserPicker from './UserPicker.svelte';
  import RecordPicker from './RecordPicker.svelte';
  import Markdown from './Markdown.svelte';
  import { get } from '../lib/api';

  let { w = $bindable(), section, editing = false }: { w: any; section: string; editing?: boolean } = $props();
  loadLists();
  let epics: any[] = $state([]);
  $effect(() => { if (editing && w.type !== 'epic' && w.space) get(`/api/spaces/${w.space}/work?type=epic`).then((r) => (epics = r)).catch(() => {}); });
  let labelsText = $state((w.labels || []).join(', '));
  $effect(() => { if (editing) w.labels = labelsText.split(',').map((s: string) => s.trim()).filter(Boolean); });
  const F = $derived(w.fields || {});
  function setF(k: string, v: any) { w.fields = { ...w.fields, [k]: v }; }
  let dispAppr: string[] = $state(w.fields?.disposition_approvers || []);
  $effect(() => { if (editing && JSON.stringify(dispAppr) !== JSON.stringify(w.fields?.disposition_approvers || [])) setF('disposition_approvers', dispAppr); });
</script>

{#snippet text(label: string, value: any)}<dt>{label}</dt><dd style="white-space: pre-wrap">{value === undefined || value === null || value === '' ? '-' : value}</dd>{/snippet}
{#snippet area(label: string, key: string)}
  {#if editing}<div class="field"><label for="w-{key}">{label}</label><textarea id="w-{key}" value={F[key] || ''} oninput={(e) => setF(key, (e.target as HTMLTextAreaElement).value)}></textarea></div>
  {:else}<dl class="props">{@render text(label, F[key])}</dl>{/if}
{/snippet}

{#if section === 'details' || section === 'report'}
  {#if editing}
    <div class="field"><label for="w-title">Title</label><input id="w-title" maxlength="150" bind:value={w.title} /></div>
    <div class="grid2">
      <div class="field"><label for="w-pri">Priority</label><select id="w-pri" bind:value={w.priority}>{#each ['Low', 'Normal', 'High', 'Urgent'] as p}<option>{p}</option>{/each}</select></div>
      <div class="field"><label for="w-asg">Assignee</label><UserPicker id="w-asg" bind:value={w.assigneeId} placeholder="Unassigned" /></div>
      <div class="field"><label for="w-due">Due date</label><input id="w-due" type="date" bind:value={w.dueDate} /></div>
      {#if w.type !== 'epic'}<div class="field"><label for="w-epic">Epic</label><select id="w-epic" bind:value={w.epic}><option value="">None</option>{#each epics as e}<option value={e.id}>{e.id} {e.title}</option>{/each}</select></div>{/if}
      {#if w.type === 'issue'}
        <div class="field"><label for="w-area">Area</label><select id="w-area" value={F.area || ''} onchange={(e) => setF('area', (e.target as HTMLSelectElement).value)}><option value="">Choose</option>{#each listOf('issue_areas') as a}<option>{a}</option>{/each}</select></div>
      {/if}
      {#if w.type === 'issue' || w.type === 'ncr'}
        <div class="field"><label for="w-sev">Severity</label><select id="w-sev" value={F.severity || ''} onchange={(e) => setF('severity', (e.target as HTMLSelectElement).value)}><option value="">Choose</option><option>Minor</option><option>Major</option><option>Critical</option></select></div>
      {/if}
      {#if w.type === 'epic'}
        <div class="field"><label for="w-sd">Start date</label><input id="w-sd" type="date" value={F.start_date || ''} oninput={(e) => setF('start_date', (e.target as HTMLInputElement).value)} /></div>
        <div class="field"><label for="w-td">Target date</label><input id="w-td" type="date" value={F.target_date || ''} oninput={(e) => setF('target_date', (e.target as HTMLInputElement).value)} /></div>
      {/if}
      {#if w.type === 'action'}<div class="field"><label for="w-src">Source page</label>{#if F.source}<div class="row"><span class="id">{F.source}</span><button class="small ghost" onclick={() => setF('source', '')}>Change</button></div>{:else}<RecordPicker kinds="page" onpick={(id) => setF('source', id)} />{/if}</div>{/if}
      {#if w.type === 'ncr'}
        <div class="field"><label for="w-fo">Found on</label>{#if F.found_on}<div class="row"><span class="id">{F.found_on}</span>{#if F.found_on_rev}<span>rev {F.found_on_rev}</span>{/if}<button class="small ghost" onclick={() => { setF('found_on', ''); setF('found_on_rev', ''); }}>Change</button></div>{:else}<RecordPicker kinds="part,assembly" onpick={(id) => setF('found_on', id)} />{/if}</div>
        <div class="field"><label for="w-q">Quantity affected</label><input id="w-q" type="number" min="0" step="any" value={F.quantity ?? ''} oninput={(e) => setF('quantity', Number((e.target as HTMLInputElement).value))} /></div>
        <div class="field"><label for="w-wf">Where found</label><select id="w-wf" value={F.where_found || ''} onchange={(e) => setF('where_found', (e.target as HTMLSelectElement).value)}><option value="">Choose</option>{#each ['Receiving inspection', 'In process', 'Final test', 'Field'] as x}<option>{x}</option>{/each}</select></div>
        <div class="field"><label for="w-sn">Serial or lot number</label><input id="w-sn" value={F.serial || ''} oninput={(e) => setF('serial', (e.target as HTMLInputElement).value)} /></div>
        <div class="field"><label for="w-mf">Manufacturer</label><select id="w-mf" value={F.manufacturer || ''} onchange={(e) => setF('manufacturer', (e.target as HTMLSelectElement).value)}><option value="">None</option>{#each lists.manufacturers as m}<option value={m.code}>{m.code} {m.name}</option>{/each}</select></div>
        <div class="field"><label for="w-po">PO or work order number</label><input id="w-po" value={F.po || ''} oninput={(e) => setF('po', (e.target as HTMLInputElement).value)} /></div>
        <div class="field"><label for="w-rq">Requirement not met</label><input id="w-rq" value={F.requirement_not_met || ''} oninput={(e) => setF('requirement_not_met', (e.target as HTMLInputElement).value)} /></div>
        <label class="checkbox"><input type="checkbox" checked={!!F.requires_design_change} onchange={(e) => setF('requires_design_change', (e.target as HTMLInputElement).checked)} /> Requires design change</label>
      {/if}
      <div class="field"><label for="w-lab">Labels (comma separated)</label><input id="w-lab" bind:value={labelsText} /></div>
    </div>
    <div class="field"><label for="w-desc">{w.type === 'ncr' ? 'Nonconformance description' : 'Description'}</label><textarea id="w-desc" rows="6" bind:value={w.description}></textarea></div>
    {#if w.type === 'ncr'}{@render area('Containment', 'containment')}{/if}
  {:else}
    <dl class="props">
      {@render text('Priority', w.priority)}
      {@render text('Assignee', w.assignee)}
      {@render text('Reporter', w.reporter)}
      {@render text('Due date', date(w.dueDate))}
      {#if w.epic}<dt>Epic</dt><dd><a class="id" href="/work/{w.epic}">{w.epic}</a></dd>{/if}
      {#if w.type === 'issue'}{@render text('Area', F.area)}{/if}
      {#if w.type === 'issue' || w.type === 'ncr'}{@render text('Severity', F.severity)}{/if}
      {#if w.type === 'epic'}{@render text('Start date', date(F.start_date))}{@render text('Target date', date(F.target_date))}{/if}
      {#if w.type === 'action' && F.source}<dt>Source</dt><dd><a class="id" href="/pages/{F.source}">{F.source}</a></dd>{/if}
      {#if w.type === 'ncr'}
        <dt>Found on</dt><dd>{#if F.found_on}<a class="id" href="/items/{encodeURIComponent(F.found_on)}{F.found_on_rev ? '?rev=' + F.found_on_rev : ''}">{F.found_on}</a>{F.found_on_rev ? ' rev ' + F.found_on_rev : ''}{:else}-{/if}</dd>
        {@render text('Quantity affected', F.quantity)}
        {@render text('Where found', F.where_found)}
        {@render text('Serial or lot number', F.serial)}
        {@render text('Manufacturer', F.manufacturer)}
        {@render text('PO or work order', F.po)}
        {@render text('Requirement not met', F.requirement_not_met)}
        {@render text('Requires design change', F.requires_design_change ? 'Yes' : 'No')}
        {@render text('Containment', F.containment)}
      {/if}
      {@render text('Labels', (w.labels || []).join(', '))}
      {@render text('Created by', w.createdBy)}
    </dl>
    <h3 style="margin-top: 1rem">{w.type === 'ncr' ? 'Nonconformance description' : 'Description'}</h3>
    {#if w.description}<Markdown source={w.description} />{:else}<div class="muted small">No description.</div>{/if}
  {/if}
{:else if section === 'disposition'}
  {#if editing}
    <div class="field"><label for="w-disp">Disposition</label><select id="w-disp" value={F.disposition || ''} onchange={(e) => setF('disposition', (e.target as HTMLSelectElement).value)}><option value="">Choose</option>{#each ['Use as is', 'Rework', 'Repair', 'Scrap', 'Return to vendor'] as x}<option>{x}</option>{/each}</select></div>
    <div class="field"><label for="w-dap">Disposition approvers</label><UserPicker multiple bind:value={dispAppr} filter={(u) => ['Super Admin', 'Release Authority', 'Approver'].includes(u.libraryRole)} placeholder="Add an approver" /></div>
  {:else}<dl class="props">{@render text('Disposition', F.disposition)}</dl>{/if}
  {@render area('Disposition notes', 'disposition_notes')}
{:else if section === 'corrective'}
  {@render area('Root cause', 'root_cause')}
  {@render area('Corrective action', 'corrective_action')}
{:else if section === 'verification'}
  {@render area('Verification of effectiveness', 'verification')}
{/if}
