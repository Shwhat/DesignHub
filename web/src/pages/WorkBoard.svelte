<script lang="ts">
  import Sortable from 'sortablejs';
  import { get, post, put } from '../lib/api';
  import { canIn, toast, onEvent, session, setSpace, spaceName } from '../lib/state.svelte';
  import { navigate } from '../lib/router.svelte';
  import { date, kindLabel } from '../lib/format';
  import { onDestroy, tick } from 'svelte';
  let { params, pattern }: { params: any; pattern?: string } = $props();
  const space = params.id;
  const ncr = pattern?.includes('ncr');
  setSpace(space);
  if (window.innerWidth <= 600) navigate(`/spaces/${space}/${ncr ? 'ncr-list' : 'list'}`, true);

  const states = ncr ? ['Reported', 'Dispositioned', 'Corrective Action', 'Verified', 'Closed'] : ['Open', 'In Progress', 'In Review', 'Blocked', 'Done', 'Cancelled'];
  let rows: any[] = $state([]);
  let lanes = $state('');
  let typeFilter = $state('');
  let sortables: Sortable[] = [];
  async function load() {
    rows = await get(`/api/spaces/${space}/work?type=${ncr ? 'ncr' : 'task,issue,action,epic'}`);
    await tick();
    bind();
  }
  get('/api/prefs/board.' + (ncr ? 'ncr' : 'work'), { background: true }).then((p) => { if (p) lanes = p.lanes || ''; }).catch(() => {});
  load();
  const off = onEvent((e) => { if (e.type === 'board' && e.space === space) load(); });
  onDestroy(() => { off(); sortables.forEach((s) => s.destroy()); });
  const canEdit = canIn(space, 'work.edit');

  function bind() {
    sortables.forEach((s) => s.destroy());
    sortables = [];
    if (!canEdit) return;
    document.querySelectorAll<HTMLElement>('.column-body').forEach((el) => {
      sortables.push(Sortable.create(el, {
        group: 'board-' + (el.dataset.lane || ''), animation: 120, sort: false,
        onEnd: async (e) => {
          const id = (e.item as HTMLElement).dataset.id!;
          const to = (e.to as HTMLElement).dataset.status!;
          const from = (e.from as HTMLElement).dataset.status!;
          if (to === from) return;
          try { await post(`/api/work/${id}/status`, { status: to }); }
          catch (err: any) { toast(err.message, 'error'); }
          load();
        },
      }));
    });
  }
  const filtered = $derived(rows.filter((r) => !typeFilter || r.type === typeFilter));
  const laneKeys = $derived.by(() => {
    if (!lanes) return [''];
    const keys = new Set<string>();
    for (const r of filtered) keys.add(laneOf(r));
    return [...keys].sort();
  });
  function laneOf(r: any) { return lanes === 'assignee' ? r.assignee || 'Unassigned' : lanes === 'epic' ? r.epic || 'No epic' : lanes === 'priority' ? r.priority : ''; }
  function savePrefs() { put('/api/prefs/board.' + (ncr ? 'ncr' : 'work'), { lanes }).catch(() => {}); tick().then(bind); }
  const icon: Record<string, string> = { task: '☑', issue: '⚠', action: '➜', epic: '◆', ncr: '✖' };
</script>

<div class="content" style="max-width: none">
  <div class="row" style="margin-bottom: 1rem">
    <h1 style="margin: 0">{ncr ? 'NCR Board' : 'Board'}</h1><span class="muted">{spaceName(space)}</span>
    <span class="spacer"></span>
    {#if !ncr}<select style="width: auto" bind:value={typeFilter} aria-label="Type"><option value="">All types</option><option value="task">Tasks</option><option value="issue">Issues</option><option value="action">Actions</option><option value="epic">Epics</option></select>{/if}
    <select style="width: auto" bind:value={lanes} onchange={savePrefs} aria-label="Swimlanes"><option value="">No swimlanes</option><option value="assignee">Swimlanes by assignee</option><option value="epic">Swimlanes by epic</option><option value="priority">Swimlanes by priority</option></select>
    {#if canEdit}<a class="btn primary" href="/work/new?space={space}&type={ncr ? 'ncr' : 'task'}">New {ncr ? 'NCR' : 'item'}</a>{/if}
  </div>
  {#each laneKeys as lane (lane)}
    {#if lane}<div class="lane-title">{lane}</div>{/if}
    <div class="board">
      {#each states as st}
        {@const cards = filtered.filter((r) => r.status === st && (!lanes || laneOf(r) === lane))}
        <div class="column">
          <div class="column-head">{st}<span class="muted small">{cards.length}</span></div>
          <div class="column-body" data-status={st} data-lane={lane}>
            {#each cards as c (c.id)}
              <div class="wcard" data-id={c.id} role="button" tabindex="0" onclick={() => navigate('/work/' + c.id)} onkeydown={(e) => e.key === 'Enter' && navigate('/work/' + c.id)}>
                <div class="row small"><span title={kindLabel(c.type)}>{icon[c.type]}</span><span class="id">{c.id}</span><span class="spacer"></span><span class="pill">{c.priority}</span></div>
                <div class="t">{c.title}</div>
                <div class="row small muted">{c.assignee || 'Unassigned'}<span class="spacer"></span>{#if c.dueDate}<span style={c.overdue ? 'color: var(--danger); font-weight: 600' : ''}>{date(c.dueDate)}</span>{/if}{#if c.blocked}<span class="status Blocked">blocked</span>{/if}</div>
              </div>
            {/each}
          </div>
        </div>
      {/each}
    </div>
  {/each}
</div>
