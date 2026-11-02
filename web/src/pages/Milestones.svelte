<script lang="ts">
  import { untrack, onDestroy } from 'svelte';
  import { get, post, patch } from '../lib/api';
  import { toast, onEvent, setSpace, spaceName, canIn, session } from '../lib/state.svelte';
  import { navigate } from '../lib/router.svelte';
  import { date, hours, addDays, today } from '../lib/format';
  import { exportCSV } from '../lib/exporting';
  import Modal from '../components/Modal.svelte';
  import Status from '../components/Status.svelte';
  import UserPicker from '../components/UserPicker.svelte';
  import Icon from '../components/Icon.svelte';

  // One page for the list (/spaces/:id/milestones) and one milestone (/milestones/:id).
  let { params, pattern }: { params: any; pattern?: string } = $props();
  const single = untrack(() => pattern === '/milestones/:id');
  const pid = untrack(() => params.id);
  let space = $state(single ? '' : pid);
  let list: any[] = $state([]);
  let one: any = $state(null);
  let error = $state('');
  let creating = $state(false);
  let editing = $state(false);
  let form = $state({ name: '', target: addDays(today(), 30), description: '', ownerId: session.me?.id || '' });

  async function load() {
    try {
      if (single) {
        one = await get('/api/milestones/' + pid);
        space = one.milestone.space;
      } else {
        list = await get(`/api/spaces/${space}/milestones`);
      }
      setSpace(space);
    } catch (e: any) { error = e.status === 404 ? 'Not found.' : e.message; }
  }
  load();
  const off = onEvent((e) => { if (e.type === 'board') load(); });
  onDestroy(off);
  const canPlan = $derived(space ? canIn(space, 'plan.manage') : false);
  async function create() {
    try { const r = await post('/api/milestones', { space, ...form }); creating = false; navigate('/milestones/' + r.id); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function save() {
    try { await patch('/api/milestones/' + pid, form); editing = false; load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  const healthClasses: Record<string, string> = { 'On track': 'Approved', 'At risk': 'In-Review', Late: 'Rejected', Done: 'Superseded' };
  const healthClass = (h: string) => healthClasses[h] || '';
</script>

{#if error}<div class="content narrow"><div class="error-box">{error}</div></div>
{:else if !single}
<div class="content">
  <div class="row" style="margin-bottom: 10px">
    <h1 style="margin: 0">Milestones</h1><span class="muted">{spaceName(space)}</span><span class="spacer"></span>
    <button class="small" onclick={() => exportCSV('Milestones ' + spaceName(space), [{ label: 'ID', value: (m: any) => m.id }, { label: 'Name', value: (m: any) => m.name }, { label: 'Target', value: (m: any) => m.target }, { label: 'Owner', value: (m: any) => m.owner }, { label: 'Health', value: (m: any) => m.health }, { label: 'Done', value: (m: any) => m.done }, { label: 'Items', value: (m: any) => m.items }], list)}>Export CSV</button>
    {#if canPlan}<button class="primary" onclick={() => (creating = true)}>New milestone...</button>{/if}
  </div>
  <div class="card table-wrap"><table class="data">
    <thead><tr><th>Name</th><th>Target</th><th>Owner</th><th>Health</th><th style="width: 200px">Progress</th><th class="num">Items</th></tr></thead>
    <tbody>
      {#each list as m (m.id)}
        <tr class="clickable" onclick={() => navigate('/milestones/' + m.id)}>
          <td><Icon name="baseline" size={14} /> <strong>{m.name}</strong></td><td>{date(m.target)}</td><td>{m.owner}</td><td><span class="status {healthClass(m.health)}">{m.health}</span></td>
          <td><div class="row" style="flex-wrap: nowrap"><div class="progress" style="flex: 1"><div style="width: {m.percent}%"></div></div><span class="small">{m.percent}%</span></div></td><td class="num">{m.done} / {m.items}</td>
        </tr>
      {:else}<tr><td colspan="6" class="empty">No milestones yet.</td></tr>{/each}
    </tbody>
  </table></div>
</div>
{:else if one}
<div class="content narrow">
  <nav class="small muted" style="margin-bottom: 4px"><a href="/spaces/{space}">{spaceName(space)}</a> / <a href="/spaces/{space}/milestones">Milestones</a></nav>
  <div class="record-head">
    <div class="titles">
      <div class="meta"><span class="id">{one.milestone.id}</span><span class="status {healthClass(one.milestone.health)}">{one.milestone.health}</span><span>Target {date(one.milestone.target)}</span>{#if one.milestone.completedOn}<span class="muted">Completed {date(one.milestone.completedOn)}</span>{/if}</div>
      <h1>{one.milestone.name}</h1>
    </div>
    {#if one.canPlan && !editing}<button onclick={() => { form = { name: one.milestone.name, target: one.milestone.target, description: one.milestone.description, ownerId: one.milestone.ownerId }; editing = true; }}>Edit</button>{/if}
  </div>
  {#if editing}
    <div class="card card-body" style="margin-bottom: 12px">
      <div class="grid2">
        <div class="field"><label for="m-n">Name</label><input id="m-n" bind:value={form.name} /></div>
        <div class="field"><label for="m-t">Target date</label><input id="m-t" type="date" bind:value={form.target} /></div>
        <div class="field"><label for="m-o">Owner</label><UserPicker id="m-o" bind:value={form.ownerId} /></div>
      </div>
      <div class="field"><label for="m-d">Description</label><textarea id="m-d" bind:value={form.description}></textarea></div>
      <div class="row end"><button onclick={() => (editing = false)}>Cancel</button><button class="primary" onclick={save}>Save</button></div>
    </div>
  {/if}
  <div class="grid2" style="margin-bottom: 12px">
    <div class="card card-body"><div class="muted small">By count</div><strong>{one.milestone.done} of {one.milestone.items} items done</strong><div class="progress" style="margin-top: 6px"><div style="width: {one.milestone.percent}%"></div></div></div>
    <div class="card card-body"><div class="muted small">By estimate</div><strong>{hours(one.milestone.doneEstimate)} of {hours(one.milestone.estimate)}</strong><div class="progress" style="margin-top: 6px"><div style="width: {one.milestone.estimate ? (one.milestone.doneEstimate * 100) / one.milestone.estimate : 0}%"></div></div></div>
  </div>
  {#if one.milestone.description}<p style="white-space: pre-wrap">{one.milestone.description}</p>{/if}
  <div class="card">
    <div class="card-head">Linked items<span class="spacer"></span><span class="muted small">Set an item's milestone in its Details</span></div>
    {#each one.items as w}<div class="comment row"><a class="id" href="/work/{w.id}"><Icon name={w.type} size={14} /> {w.id}</a><span style="flex: 1">{w.title}</span><span class="muted small">{w.assignee}</span><Status value={w.status} /></div>
    {:else}<div class="empty">No items are linked to this milestone.</div>{/each}
  </div>
</div>
{/if}

{#if creating}
  <Modal title="New milestone" onclose={() => (creating = false)}>
    <div class="field"><label for="n-n">Name</label><input id="n-n" maxlength="80" bind:value={form.name} /></div>
    <div class="grid2">
      <div class="field"><label for="n-t">Target date</label><input id="n-t" type="date" bind:value={form.target} /></div>
      <div class="field"><label for="n-o">Owner</label><UserPicker id="n-o" bind:value={form.ownerId} /></div>
    </div>
    <div class="field"><label for="n-d">Description</label><textarea id="n-d" bind:value={form.description}></textarea></div>
    {#snippet footer()}<button onclick={() => (creating = false)}>Cancel</button><button class="primary" onclick={create}>Create</button>{/snippet}
  </Modal>
{/if}
