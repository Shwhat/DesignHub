<script lang="ts">
  import { untrack } from 'svelte';
  import { get } from '../lib/api';
  import { toast, setSpace, spaceName } from '../lib/state.svelte';
  import { today, addDays, hours } from '../lib/format';
  import { exportCSV, fullExport } from '../lib/exporting';
  import UserPicker from '../components/UserPicker.svelte';

  let { params }: { params: any; pattern?: string } = $props();
  const space = untrack(() => params.id);
  setSpace(space);
  let from = $state(addDays(today(), -6));
  let to = $state(today());
  let group = $state('person');
  let people: string[] = $state([]);
  let wtype = $state('');
  let sprint = $state('');
  let milestone = $state('');
  let sprints: any[] = $state([]);
  let milestones: any[] = $state([]);
  let d: any = $state(null);

  get(`/api/spaces/${space}/sprints`).then((r) => (sprints = r)).catch(() => {});
  get(`/api/spaces/${space}/milestones`).then((r) => (milestones = r)).catch(() => {});
  async function load() {
    const q = new URLSearchParams({ from, to, group });
    if (people.length) q.set('people', people.join(','));
    if (wtype) q.set('type', wtype);
    if (sprint) q.set('sprint', sprint);
    if (milestone) q.set('milestone', milestone);
    try { d = await get(`/api/spaces/${space}/time-report?` + q); } catch (e: any) { toast(e.message, 'error'); }
  }
  $effect(() => { from; to; group; people; wtype; sprint; milestone; load(); });
  const groupLabels: Record<string, string> = { person: 'Person', item: 'Item', epic: 'Epic', sprint: 'Sprint' };
  const groupLabel = $derived(groupLabels[group] || 'Person');
  function exportTable() {
    exportCSV(`Time report ${spaceName(space)} ${from} to ${to}`, [
      { label: groupLabel, value: (r: any) => r.label }, { label: 'Estimate', value: (r: any) => r.estimate }, { label: 'Logged', value: (r: any) => r.logged },
      { label: 'Difference', value: (r: any) => r.difference }, { label: 'Entries', value: (r: any) => r.entries },
    ], [...d.rows, d.total]);
  }
</script>

<div class="content">
  <div class="row" style="margin-bottom: 10px">
    <h1 style="margin: 0">Time report</h1><span class="muted">{spaceName(space)}</span><span class="spacer"></span>
    {#if d}<button class="small" onclick={exportTable}>Export CSV</button><button class="small" onclick={() => fullExport('time', `space=${space}&from=${from}&to=${to}`)}>Export entries (CSV)</button>{/if}
  </div>
  <div class="card card-body" style="margin-bottom: 12px">
    <div class="row" style="align-items: flex-end">
      <div style="width: 150px"><label for="r-f">From</label><input id="r-f" type="date" bind:value={from} /></div>
      <div style="width: 150px"><label for="r-t">To</label><input id="r-t" type="date" bind:value={to} /></div>
      <div style="width: 130px"><label for="r-g">Group by</label><select id="r-g" bind:value={group}><option value="person">Person</option><option value="item">Item</option><option value="epic">Epic</option><option value="sprint">Sprint</option></select></div>
      <div style="width: 120px"><label for="r-y">Type</label><select id="r-y" bind:value={wtype}><option value="">Any</option><option value="task">Task</option><option value="issue">Issue</option><option value="action">Action</option><option value="ncr">NCR</option></select></div>
      <div style="width: 160px"><label for="r-s">Sprint</label><select id="r-s" bind:value={sprint}><option value="">Any</option>{#each sprints as s}<option value={s.id}>{s.name}</option>{/each}</select></div>
      <div style="width: 160px"><label for="r-m">Milestone</label><select id="r-m" bind:value={milestone}><option value="">Any</option>{#each milestones as m}<option value={m.id}>{m.name}</option>{/each}</select></div>
      <div style="flex: 1; min-width: 200px"><label for="r-p">People</label><UserPicker id="r-p" multiple bind:value={people} placeholder="Everyone" /></div>
    </div>
  </div>
  {#if d}
    <div class="card table-wrap">
      <table class="data">
        <thead><tr><th>{groupLabel}</th><th class="num">Estimate</th><th class="num">Logged</th><th class="num">Difference</th><th class="num">Entries</th></tr></thead>
        <tbody>
          {#each d.rows as r}<tr><td>{r.label}</td><td class="num">{hours(r.estimate)}</td><td class="num">{hours(r.logged)}</td><td class="num" style:color={r.difference > 0 ? 'var(--err)' : ''}>{hours(r.difference)}</td><td class="num">{r.entries}</td></tr>
          {:else}<tr><td colspan="5" class="empty">No time logged in this range.</td></tr>{/each}
          {#if d.rows.length}<tr style="font-weight: 600"><td>Total</td><td class="num">{hours(d.total.estimate)}</td><td class="num">{hours(d.total.logged)}</td><td class="num">{hours(d.total.difference)}</td><td class="num">{d.total.entries}</td></tr>{/if}
        </tbody>
      </table>
    </div>
    <p class="small muted">Estimates count once per item, for items with time in the range. A positive difference means more time was logged than estimated.</p>
  {/if}
</div>
