<script lang="ts">
  import { untrack, onDestroy } from 'svelte';
  import { get, post } from '../lib/api';
  import { toast, onEvent, setSpace, spaceName, canIn } from '../lib/state.svelte';
  import { navigate } from '../lib/router.svelte';
  import { date, hours, today, addDays } from '../lib/format';
  import { exportCSV } from '../lib/exporting';
  import Modal from '../components/Modal.svelte';
  import Status from '../components/Status.svelte';
  import Icon from '../components/Icon.svelte';

  let { params }: { params: any; pattern?: string } = $props();
  const space = untrack(() => params.id);
  setSpace(space);
  let sprints: any[] = $state([]);
  let backlog: any[] = $state([]);
  let selected: string[] = $state([]);
  let creating = $state(false);
  let form = $state({ name: '', goal: '', start: today(), end: addDays(today(), 13) });
  let closing: any = $state(null);
  let moveTo = $state('');
  let dragId = $state('');
  const canPlan = canIn(space, 'plan.manage');
  const canEdit = canIn(space, 'work.edit');

  async function load() {
    sprints = await get(`/api/spaces/${space}/sprints`);
    const open = await get(`/api/spaces/${space}/work?open=1&type=task,issue,action,ncr`);
    backlog = open.filter((w: any) => !w.sprint);
  }
  load();
  const off = onEvent((e) => { if (e.type === 'board' && (!e.space || e.space === space)) load(); });
  onDestroy(off);

  async function create() {
    try {
      await post('/api/sprints', { space, ...form });
      creating = false;
      form = { name: '', goal: '', start: today(), end: addDays(today(), 13) };
      load();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  async function start(s: any) {
    if (!confirm(`Start ${s.name}? Its current items are recorded as committed.`)) return;
    try { await post(`/api/sprints/${s.id}/start`); toast(`${s.name} started.`); load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  function openClose(s: any) {
    closing = s;
    moveTo = sprints.find((x) => x.state === 'Planned')?.id || '';
  }
  async function close() {
    try { await post(`/api/sprints/${closing.id}/close`, { moveTo }); toast(`${closing.name} closed.`); closing = null; load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function plan(items: string[], sprint: string) {
    if (!items.length) return;
    try { await post('/api/plan', { items, sprint }); selected = []; load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  function toggle(id: string) { selected = selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id]; }
  const open = $derived(sprints.filter((s) => s.state !== 'Closed'));
  function exportTable() {
    exportCSV('Sprints ' + spaceName(space), [
      { label: 'ID', value: (s: any) => s.id }, { label: 'Name', value: (s: any) => s.name }, { label: 'State', value: (s: any) => s.state },
      { label: 'Start', value: (s: any) => s.start }, { label: 'End', value: (s: any) => s.end }, { label: 'Goal', value: (s: any) => s.goal },
      { label: 'Items', value: (s: any) => s.items }, { label: 'Done', value: (s: any) => s.done }, { label: 'Estimate', value: (s: any) => s.estimate }, { label: 'Logged', value: (s: any) => s.logged },
    ], sprints);
  }
</script>

<div class="content">
  <div class="row" style="margin-bottom: 10px">
    <h1 style="margin: 0">Sprints</h1><span class="muted">{spaceName(space)}</span>
    <span class="spacer"></span>
    <button class="small" onclick={exportTable}>Export CSV</button>
    {#if canPlan}<button class="primary" onclick={() => (creating = true)}>New sprint...</button>{/if}
  </div>
  <div class="card table-wrap">
    <table class="data">
      <thead><tr><th>Name</th><th>State</th><th>Dates</th><th>Goal</th><th class="num">Items</th><th class="num">Done</th><th class="num">Estimate</th><th class="num">Logged</th><th></th></tr></thead>
      <tbody>
        {#each sprints as s (s.id)}
          <tr class="clickable" class:drop={dragId && s.state !== 'Closed'} onclick={() => navigate('/sprints/' + s.id)}
            ondragover={(e) => { if (dragId && s.state !== 'Closed') e.preventDefault(); }} ondrop={(e) => { e.preventDefault(); plan([dragId], s.id); dragId = ''; }}>
            <td><strong>{s.name}</strong></td><td><Status value={s.state === 'Active' ? 'In Progress' : s.state === 'Closed' ? 'Closed' : 'Open'} /> <span class="small muted">{s.state}</span></td>
            <td style="white-space: nowrap">{date(s.start)} to {date(s.end)}</td><td>{s.goal}</td><td class="num">{s.items}</td><td class="num">{s.done}</td>
            <td class="num">{hours(s.estimate)}</td><td class="num">{hours(s.logged)}</td>
            <td style="white-space: nowrap" onclick={(e) => e.stopPropagation()}>
              {#if canPlan && s.state === 'Planned'}<button class="small" onclick={() => start(s)}>Start</button>{/if}
              {#if canPlan && s.state === 'Active'}<button class="small" onclick={() => openClose(s)}>Close...</button>{/if}
            </td>
          </tr>
        {:else}<tr><td colspan="9" class="empty">No sprints yet.</td></tr>{/each}
      </tbody>
    </table>
  </div>

  <div class="row" style="margin: 16px 0 6px">
    <h2 style="margin: 0">Backlog</h2><span class="muted small">Open items with no sprint. Drag an item onto a sprint, or select items and add them.</span>
    <span class="spacer"></span>
    {#if canEdit && selected.length}
      <select style="width: auto" onchange={(e) => { plan(selected, (e.target as HTMLSelectElement).value); (e.target as HTMLSelectElement).value = ''; }}>
        <option value="">Add {selected.length} to sprint...</option>{#each open as s}<option value={s.id}>{s.name}</option>{/each}
      </select>
    {/if}
  </div>
  <div class="card table-wrap">
    <table class="data">
      <thead><tr><th style="width: 28px"></th><th>ID</th><th>Title</th><th>Status</th><th>Priority</th><th>Assignee</th><th class="num">Estimate</th></tr></thead>
      <tbody>
        {#each backlog as w (w.id)}
          <tr draggable={canEdit} ondragstart={() => (dragId = w.id)} ondragend={() => (dragId = '')}>
            <td>{#if canEdit}<input type="checkbox" checked={selected.includes(w.id)} onchange={() => toggle(w.id)} aria-label="Select {w.id}" />{/if}</td>
            <td><a class="id" href="/work/{w.id}"><Icon name={w.type} size={14} /> {w.id}</a></td><td>{w.title}</td><td><Status value={w.status} /></td><td>{w.priority}</td><td>{w.assignee}</td><td class="num">{hours(w.estimate)}</td>
          </tr>
        {:else}<tr><td colspan="7" class="empty">The backlog is empty.</td></tr>{/each}
      </tbody>
    </table>
  </div>
</div>

{#if creating}
  <Modal title="New sprint" onclose={() => (creating = false)}>
    <div class="field"><label for="s-n">Name</label><input id="s-n" maxlength="60" bind:value={form.name} /></div>
    <div class="field"><label for="s-g">Goal</label><input id="s-g" bind:value={form.goal} /></div>
    <div class="grid2">
      <div class="field"><label for="s-s">Start date</label><input id="s-s" type="date" bind:value={form.start} /></div>
      <div class="field"><label for="s-e">End date</label><input id="s-e" type="date" bind:value={form.end} /></div>
    </div>
    {#snippet footer()}<button onclick={() => (creating = false)}>Cancel</button><button class="primary" onclick={create}>Create</button>{/snippet}
  </Modal>
{/if}
{#if closing}
  <Modal title="Close {closing.name}" onclose={() => (closing = null)}>
    <p>{closing.done} of {closing.items} items are done. Where should unfinished items go?</p>
    <select bind:value={moveTo}>
      {#each sprints.filter((x) => x.state === 'Planned') as s}<option value={s.id}>Next sprint: {s.name}</option>{/each}
      <option value="">No sprint (backlog)</option>
    </select>
    {#snippet footer()}<button onclick={() => (closing = null)}>Cancel</button><button class="primary" onclick={close}>Close sprint</button>{/snippet}
  </Modal>
{/if}

<style>
  tr.drop td { background: var(--accent-soft); }
</style>
