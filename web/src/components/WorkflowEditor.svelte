<script lang="ts">
  import { get, put, ApiError } from '../lib/api';
  import { toast } from '../lib/state.svelte';
  import { dateTime } from '../lib/format';
  import Modal from './Modal.svelte';
  import Icon from './Icon.svelte';

  // Admin, Workflows (SCREENS.md 26). One workflow per work item type.
  const types = [['task', 'Task'], ['issue', 'Issue'], ['action', 'Action'], ['epic', 'Epic'], ['ncr', 'NCR']];
  const cats = [['open', 'Open'], ['active', 'Active'], ['done', 'Done'], ['cancelled', 'Cancelled']];
  const colors = ['grey', 'blue', 'amber', 'green', 'red'];
  const swatch: Record<string, string> = { grey: '#8a94a3', blue: '#1f5fbf', amber: '#e0a100', green: '#2f7d32', red: '#b3261e' };
  let type = $state('task');
  let data: any = $state(null);
  let wf: any = $state(null);
  let roles: string[] = $state([]);
  let dirty = $state(false);
  let mapping: Record<string, string> | null = $state(null);
  let removed: Record<string, number> = $state({});
  let history: any[] = $state([]);
  let showHistory = $state(false);
  let cell: { from: string; to: string } | null = $state(null);

  async function load() {
    data = await get('/api/workflows');
    const r = await get('/api/roles');
    roles = r.roles.filter((x: any) => x.appliesTo !== 'library' && x.active).map((x: any) => x.name);
    pick(type);
  }
  load();
  function pick(t: string) {
    if (dirty && !confirm('Discard unsaved workflow changes?')) return;
    type = t;
    wf = JSON.parse(JSON.stringify(data.workflows[t]));
    dirty = false;
    showHistory = false;
  }
  const fields = $derived(data?.fields[type] || []);
  const label = (k: string) => data?.labels[k] || k;
  const tr = (from: string, to: string) => wf.transitions.find((t: any) => t.from === from && t.to === to);
  function toggle(from: string, to: string) {
    const t = tr(from, to);
    if (t) wf.transitions = wf.transitions.filter((x: any) => x !== t);
    else wf.transitions = [...wf.transitions, { from, to, roles: [], assignee: true }];
    dirty = true;
  }
  function addState() {
    let n = 'New state';
    for (let i = 2; wf.states.some((s: any) => s.name === n); i++) n = 'New state ' + i;
    wf.states = [...wf.states, { name: n, category: 'active', color: 'blue', required: [] }];
    dirty = true;
  }
  function removeState(i: number) {
    const s = wf.states[i];
    wf.states = wf.states.filter((_: any, j: number) => j !== i);
    wf.transitions = wf.transitions.filter((t: any) => t.from !== s.name && t.to !== s.name);
    dirty = true;
  }
  function move(i: number, d: number) {
    const j = i + d;
    if (j < 0 || j >= wf.states.length) return;
    const a = [...wf.states];
    [a[i], a[j]] = [a[j], a[i]];
    wf.states = a;
    dirty = true;
  }
  function rename(i: number, name: string) {
    const old = wf.states[i].name;
    wf.states[i].name = name;
    for (const t of wf.transitions) {
      if (t.from === old) t.from = name;
      if (t.to === old) t.to = name;
    }
    if (wf.initial === old) wf.initial = name;
    dirty = true;
  }
  async function save(map?: Record<string, string>) {
    try {
      await put('/api/workflows/' + type, { workflow: wf, version: data.workflows[type].version, mapping: map });
      toast(`${types.find((t) => t[0] === type)?.[1]} workflow saved.`);
      mapping = null;
      dirty = false;
      await load();
    } catch (e: any) {
      if (e instanceof ApiError && e.code === 'mapping') {
        removed = e.data;
        mapping = Object.fromEntries(Object.keys(e.data).map((k) => [k, wf.initial]));
      } else toast(e.message, 'error');
    }
  }
  async function loadHistory() {
    history = await get(`/api/workflows/${type}/history`);
    showHistory = true;
  }
  const names = $derived((wf?.states || []).map((s: any) => s.name));
</script>

{#if wf}
  <div class="row" style="margin-bottom: 10px">
    <span class="seg">{#each types as [k, l]}<button class:on={type === k} onclick={() => pick(k)}>{l}</button>{/each}</span>
    <span class="muted small">Version {data.workflows[type].version}{data.workflows[type].changedBy ? ', changed by ' + data.workflows[type].changedBy + ' ' + dateTime(data.workflows[type].changedAt) : ''}</span>
    <span class="spacer"></span>
    <button class="small" onclick={loadHistory}>History</button>
    <button class="small" disabled={!dirty} onclick={() => pick(type)}>Discard</button>
    <button class="primary" disabled={!dirty} onclick={() => save()}>Save workflow</button>
  </div>
  <p class="small muted">The workflow applies in every space. Categories decide what counts as open and done in My Work, progress, sprints and reports. Change order states are fixed and not edited here.</p>

  <div class="card" style="margin-bottom: 12px">
    <div class="card-head">States<span class="spacer"></span><button class="small" onclick={addState}><Icon name="plus" size={14} />Add state</button></div>
    <div class="table-wrap"><table class="data">
      <thead><tr><th style="width: 54px"></th><th>Name</th><th>Category</th><th>Color</th><th>Required to enter</th><th>Initial</th><th></th></tr></thead>
      <tbody>
        {#each wf.states as s, i (i)}
          <tr>
            <td style="white-space: nowrap"><button class="small ghost icon" aria-label="Move up" onclick={() => move(i, -1)}><Icon name="up" size={12} /></button><button class="small ghost icon" aria-label="Move down" onclick={() => move(i, 1)}><Icon name="dn" size={12} /></button></td>
            <td><input value={s.name} maxlength="30" onchange={(e) => rename(i, (e.target as HTMLInputElement).value.trim())} aria-label="State name" />{#if s.rule === 'disposition'}<div class="small muted"><Icon name="lock" size={11} /> Disposition: signed approvals required, cannot be removed</div>{/if}</td>
            <td><select bind:value={s.category} onchange={() => (dirty = true)} aria-label="Category">{#each cats as [k, l]}<option value={k}>{l}</option>{/each}</select></td>
            <td><select bind:value={s.color} onchange={() => (dirty = true)} aria-label="Color" style="border-left: 6px solid {swatch[s.color]}">{#each colors as c}<option>{c}</option>{/each}</select></td>
            <td><select multiple size="3" bind:value={s.required} onchange={() => (dirty = true)} aria-label="Required fields" style="min-width: 160px">{#each fields as f}<option value={f}>{label(f)}</option>{/each}</select></td>
            <td><input type="radio" name="initial" checked={wf.initial === s.name} disabled={s.category !== 'open'} onchange={() => { wf.initial = s.name; dirty = true; }} aria-label="Initial state" /></td>
            <td>{#if s.rule !== 'disposition'}<button class="small ghost danger" onclick={() => removeState(i)}>Remove</button>{/if}</td>
          </tr>
        {/each}
      </tbody>
    </table></div>
    {#if type === 'ncr'}<div class="card-body note">Fixed NCR rules always apply: the Disposition state needs a disposition with every approver signed, and Done states need a root cause, a corrective action, and a linked change order when a design change is required.</div>{/if}
  </div>

  <div class="card" style="margin-bottom: 12px">
    <div class="card-head">Transitions<span class="spacer"></span><span class="small muted">Rows are the current state, columns the next state. Click a dot to change who may move.</span></div>
    <div class="table-wrap"><table class="data grid">
      <thead><tr><th>From</th>{#each names as to}<th style="text-align: center">{to}</th>{/each}</tr></thead>
      <tbody>
        {#each ['*', ...names] as from}
          <tr>
            <td><strong>{from === '*' ? 'Any state' : from}</strong></td>
            {#each names as to}
              <td style="text-align: center">
                {#if from !== to}
                  {@const t = tr(from, to)}
                  <input type="checkbox" checked={!!t} onchange={() => toggle(from, to)} aria-label="{from} to {to}" />
                  {#if t}<button class="lnk small" onclick={() => (cell = { from, to })}>{t.roles.length ? t.roles.join(', ') : 'anyone'}{t.assignee && t.roles.length ? ', assignee' : ''}</button>{/if}
                {/if}
              </td>
            {/each}
          </tr>
        {/each}
      </tbody>
    </table></div>
  </div>

  <div class="card card-body">
    <div class="small muted" style="margin-bottom: 6px">Preview</div>
    <div class="flow">
      {#each wf.states as s}
        <div class="st" style="border-color: {swatch[s.color]}"><strong>{s.name}</strong><span class="small muted">{cats.find((c) => c[0] === s.category)?.[1]}</span>
          <span class="small">to {wf.transitions.filter((t: any) => t.from === s.name || t.from === '*').map((t: any) => t.to).filter((x: string) => x !== s.name).join(', ') || 'nothing'}</span></div>
      {/each}
    </div>
  </div>
{/if}

{#if cell}
  {@const t = tr(cell.from, cell.to)}
  <Modal title="{cell.from === '*' ? 'Any state' : cell.from} to {cell.to}" onclose={() => (cell = null)}>
    <p class="small muted">Space roles allowed to make this move. None selected means anyone who can edit work items in the space.</p>
    {#each roles as r}<label class="checkbox" style="margin-bottom: 4px"><input type="checkbox" checked={t.roles.includes(r)} onchange={(e) => { t.roles = (e.target as HTMLInputElement).checked ? [...t.roles, r] : t.roles.filter((x: string) => x !== r); dirty = true; }} /> {r}</label>{/each}
    <label class="checkbox" style="margin-top: 8px"><input type="checkbox" bind:checked={t.assignee} onchange={() => (dirty = true)} /> The assignee may always make this move</label>
    {#snippet footer()}<button class="primary" onclick={() => (cell = null)}>Done</button>{/snippet}
  </Modal>
{/if}
{#if mapping}
  <Modal title="Move items out of removed states" onclose={() => (mapping = null)}>
    <p>These states hold items. Choose a new state for each before saving.</p>
    {#each Object.entries(removed) as [st, n]}
      <div class="field"><label for="map-{st}">{st} ({n} items)</label><select id="map-{st}" bind:value={mapping[st]}>{#each names as s}<option>{s}</option>{/each}</select></div>
    {/each}
    {#snippet footer()}<button onclick={() => (mapping = null)}>Cancel</button><button class="primary" onclick={() => save(mapping || undefined)}>Move items and save</button>{/snippet}
  </Modal>
{/if}
{#if showHistory}
  <Modal title="Workflow history" onclose={() => (showHistory = false)}>
    <table class="data"><thead><tr><th>Version</th><th>Changed by</th><th>When</th><th>States</th></tr></thead><tbody>
      {#each history as h}<tr><td>{h.version}</td><td>{h.by}</td><td>{dateTime(h.at)}</td><td class="small">{h.definition.states.map((s: any) => s.name).join(', ')}</td></tr>{/each}
    </tbody></table>
  </Modal>
{/if}

<style>
  .flow { display: flex; flex-wrap: wrap; gap: 8px; }
  .st { border: 1px solid; border-left-width: 4px; border-radius: 4px; padding: 5px 9px; display: flex; flex-direction: column; min-width: 130px; background: var(--panel2); }
  table.grid td { padding: 3px 6px; }
</style>
