<script lang="ts">
  import { post } from '../lib/api';
  import { session, toast, canIn } from '../lib/state.svelte';
  import { navigate, route } from '../lib/router.svelte';
  import WorkFields from '../components/WorkFields.svelte';
  let { params }: { params?: any; pattern?: string } = $props();
  const space = route.query.get('space') || session.space;
  let w: any = $state({ type: route.query.get('type') || 'task', space, title: '', description: '', priority: 'Normal', assigneeId: '', dueDate: '', epic: '', labels: [], fields: {}, attachments: [] });
  let busy = $state(false);
  async function create() {
    busy = true;
    try {
      const r = await post('/api/work', { type: w.type, space: w.space, title: w.title, description: w.description, priority: w.priority, assigneeId: w.assigneeId || '', dueDate: w.dueDate || '', epic: w.epic, labels: w.labels, fields: w.fields });
      toast(`${r.id} created.`);
      navigate('/work/' + r.id, true);
    } catch (e: any) { toast(e.message, 'error'); }
    busy = false;
  }
</script>

<div class="content narrow">
  <h1>New work item</h1>
  {#if !canIn(w.space, 'work.edit')}<div class="error-box">You cannot create work items in this space.</div>{:else}
  <div class="card card-body">
    <div class="grid2">
      <div class="field"><label for="wt">Type</label><select id="wt" bind:value={w.type} onchange={() => (w.fields = {})}><option value="task">Task</option><option value="issue">Issue</option><option value="action">Action</option><option value="epic">Epic</option><option value="ncr">Nonconformance</option></select></div>
      <div class="field"><label for="ws">Space</label><select id="ws" bind:value={w.space}>{#each (session.me?.spaces || []).filter((s) => s.permissions?.includes('work.edit') && !s.archived) as s}<option value={s.id}>{s.name}</option>{/each}</select></div>
    </div>
    {#key w.type}<WorkFields bind:w section="details" editing />{/key}
    <div class="row end"><button onclick={() => history.back()}>Cancel</button><button class="primary" disabled={busy || !w.title.trim()} onclick={create}>Create</button></div>
  </div>
  {/if}
</div>
