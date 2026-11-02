<script lang="ts">
  import { untrack, onDestroy } from 'svelte';
  import { get, patch } from '../lib/api';
  import { toast, onEvent, setSpace, spaceName } from '../lib/state.svelte';
  import { date, today, addDays } from '../lib/format';
  import Icon from '../components/Icon.svelte';

  let { params }: { params: any; pattern?: string } = $props();
  const space = untrack(() => params.id);
  setSpace(space);
  let d: any = $state(null);
  let group = $state<'epic' | 'milestone'>('epic');
  let scale = $state<'week' | 'month' | 'quarter'>('month');
  let undo: { id: string; type: string; field: string; value: string }[] = $state([]);
  let drag: { id: string; startX: number; orig: string; type: string } | null = $state(null);
  let dragDays = $state(0);

  async function load() {
    try { d = await get(`/api/spaces/${space}/timeline`); } catch (e: any) { toast(e.message, 'error'); }
  }
  load();
  const off = onEvent((e) => { if (e.type === 'board' && e.space === space) load(); });
  onDestroy(off);

  const px = $derived(scale === 'week' ? 26 : scale === 'month' ? 8 : 3);
  const toN = (s: string) => { const [y, m, dd] = s.split('-').map(Number); return Math.round(Date.UTC(y, m - 1, dd) / 86400000); };
  const fromN = (n: number) => new Date(n * 86400000).toISOString().slice(0, 10);
  const range = $derived.by(() => {
    if (!d) return { start: toN(today()) - 14, end: toN(today()) + 60 };
    const ds = [today()];
    for (const b of d.bars) ds.push(b.start, b.end);
    for (const m of d.milestones) ds.push(m.target);
    for (const s of d.sprints) ds.push(s.start, s.end);
    const ns = ds.map(toN);
    return { start: Math.min(...ns) - 7, end: Math.max(...ns) + 14 };
  });
  const width = $derived((range.end - range.start) * px);
  const x = (s: string) => (toN(s) - range.start) * px;
  const ticks = $derived.by(() => {
    const out: { x: number; label: string }[] = [];
    for (let n = range.start; n <= range.end; n++) {
      const s = fromN(n);
      const day = Number(s.slice(8));
      if (scale === 'week' ? new Date(n * 86400000).getUTCDay() === 1 : day === 1) {
        out.push({ x: (n - range.start) * px, label: scale === 'week' ? date(s) : new Date(n * 86400000).toLocaleDateString(undefined, { month: 'short', year: scale === 'quarter' ? '2-digit' : undefined, timeZone: 'UTC' }) });
      }
    }
    return out;
  });
  const groups = $derived.by(() => {
    if (!d) return [];
    const out: { key: string; label: string; href: string; bars: any[]; marker?: any }[] = [];
    if (group === 'epic') {
      for (const e of d.bars.filter((b: any) => b.type === 'epic')) out.push({ key: e.id, label: e.id + ' ' + e.title, href: '/work/' + e.id, bars: [e, ...d.bars.filter((b: any) => b.epic === e.id)] });
      const rest = d.bars.filter((b: any) => b.type !== 'epic' && !out.some((g) => g.key === b.epic));
      if (rest.length) out.push({ key: '', label: 'No epic', href: '', bars: rest });
    } else {
      for (const m of d.milestones) out.push({ key: m.id, label: m.name, href: '/milestones/' + m.id, bars: d.bars.filter((b: any) => b.milestone === m.id), marker: m });
      const rest = d.bars.filter((b: any) => !b.milestone);
      if (rest.length) out.push({ key: '', label: 'No milestone', href: '', bars: rest });
    }
    return out;
  });
  // Row centers for drawing dependency arrows: a group header row, then one row per bar.
  const rowY = $derived.by(() => {
    const m: Record<string, number> = {};
    let y = 40;
    for (const g of groups) {
      y += 26;
      for (const b of g.bars) { if (!(b.id in m)) m[b.id] = y + 13; y += 26; }
    }
    return { m, height: y };
  });
  const color = (b: any) => (b.category === 'done' ? 'var(--ok)' : b.type === 'epic' ? '#7a5cc0' : b.overdue ? 'var(--err)' : 'var(--accent)');

  function startDrag(e: PointerEvent, b: any) {
    if (!d.canEdit) return;
    e.preventDefault();
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
    drag = { id: b.id, startX: e.clientX, orig: b.end, type: b.type };
    dragDays = 0;
  }
  function moveDrag(e: PointerEvent) {
    if (drag) dragDays = Math.round((e.clientX - drag.startX) / px);
  }
  async function endDrag() {
    if (!drag) return;
    const dr = drag;
    drag = null;
    if (!dragDays) return;
    const value = addDays(dr.orig, dragDays);
    await save(dr.id, dr.type, value, dr.orig);
  }
  async function save(id: string, type: string, value: string, before?: string) {
    try {
      if (type === 'epic') await patch('/api/work/' + id, { fields: { target_date: value } });
      else await patch('/api/work/' + id, { dueDate: value });
      if (before) undo = [...undo, { id, type, field: type === 'epic' ? 'target' : 'due', value: before }];
      toast(`${id} now ends ${date(value)}. Press Ctrl+Z to undo.`);
      load();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  function key(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'z' && undo.length && !(e.target as HTMLElement).closest('input, textarea')) {
      e.preventDefault();
      const u = undo[undo.length - 1];
      undo = undo.slice(0, -1);
      save(u.id, u.type, u.value);
    }
  }
  const byMonth = $derived.by(() => {
    const m: Record<string, any[]> = {};
    for (const b of d?.bars || []) (m[b.end.slice(0, 7)] ||= []).push(b);
    return Object.entries(m).sort();
  });
  const endOf = (b: any) => (drag && drag.id === b.id ? addDays(b.end, dragDays) : b.end);
</script>

<svelte:window onkeydown={key} />
<div class="content" style="max-width: none">
  <div class="row" style="margin-bottom: 10px">
    <h1 style="margin: 0">Timeline</h1><span class="muted">{spaceName(space)}</span><span class="spacer"></span>
    <span class="seg hide-phone">{#each [['epic', 'By epic'], ['milestone', 'By milestone']] as [k, l]}<button class:on={group === k} onclick={() => (group = k as any)}>{l}</button>{/each}</span>
    <span class="seg hide-phone">{#each [['week', 'Week'], ['month', 'Month'], ['quarter', 'Quarter']] as [k, l]}<button class:on={scale === k} onclick={() => (scale = k as any)}>{l}</button>{/each}</span>
  </div>
  {#if d}
    <div class="phone-only">
      {#each byMonth as [m, bars]}
        <h3>{new Date(m + '-01T00:00:00').toLocaleDateString(undefined, { month: 'long', year: 'numeric' })}</h3>
        <div class="card" style="margin-bottom: 10px">{#each bars as b}<a class="comment row" href="/work/{b.id}"><Icon name={b.type} size={14} /><span class="id">{b.id}</span><span style="flex: 1">{b.title}</span><span class="small muted">{date(b.end)}</span></a>{/each}</div>
      {/each}
      {#each d.milestones as m}<div class="small"><Icon name="baseline" size={12} /> {m.name}: {date(m.target)} ({m.health})</div>{/each}
    </div>
    <div class="card tl hide-phone">
      <div class="labels">
        <div class="lhead">Item</div>
        {#each groups as g}
          <div class="lgroup">{#if g.href}<a href={g.href}>{g.label}</a>{:else}{g.label}{/if}</div>
          {#each g.bars as b}<div class="lrow"><a href="/work/{b.id}"><Icon name={b.type} size={12} /> {b.id}</a> <span class="muted">{b.title}</span></div>{/each}
        {/each}
      </div>
      <div class="chart">
        <div style="width: {width}px; position: relative">
          <div class="thead">
            {#each d.sprints as s}<div class="sprint" style="left: {x(s.start)}px; width: {x(s.end) - x(s.start)}px" title="{s.name} ({s.state})">{s.name}</div>{/each}
            {#each ticks as t}<div class="tick" style="left: {t.x}px">{t.label}</div>{/each}
          </div>
          {#each ticks as t}<div class="grid" style="left: {t.x}px"></div>{/each}
          <div class="today" style="left: {x(today())}px" title="Today"></div>
          {#each d.milestones as m}<div class="ms" style="left: {x(m.target) - 6}px" title="{m.name}: {date(m.target)} ({m.health})"></div>{/each}
          {#each groups as g}
            <div class="cgroup"></div>
            {#each g.bars as b}
              <div class="crow">
                <div class="bar" style="left: {x(b.start)}px; width: {Math.max(px, x(endOf(b)) - x(b.start) + px)}px; background: {color(b)}" title="{b.id} {b.title}: {date(b.start)} to {date(endOf(b))}">
                  {#if d.canEdit}<span class="handle" role="slider" aria-label="Change end date of {b.id}" aria-valuenow={toN(endOf(b))} tabindex="0" onpointerdown={(e) => startDrag(e, b)} onpointermove={moveDrag} onpointerup={endDrag}></span>{/if}
                </div>
              </div>
            {/each}
          {/each}
          <svg class="deps" width={width} height={rowY.height} aria-hidden="true">
            <defs><marker id="arr" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L8 4L0 8z" fill="var(--muted)" /></marker></defs>
            {#each d.dependencies as dep}
              {@const fb = d.bars.find((b: any) => b.id === dep.from)}{@const tb = d.bars.find((b: any) => b.id === dep.to)}
              {#if fb && tb && rowY.m[fb.id] && rowY.m[tb.id]}
                {@const x1 = x(endOf(fb)) + px}{@const x2 = x(tb.start)}{@const y1 = rowY.m[fb.id]}{@const y2 = rowY.m[tb.id]}
                <path d="M{x1} {y1} H{Math.max(x1 + 6, x2 - 6)} V{y2} H{x2}" fill="none" stroke="var(--muted)" stroke-dasharray="3 2" marker-end="url(#arr)"><title>{dep.from} blocks {dep.to}</title></path>
              {/if}
            {/each}
          </svg>
        </div>
      </div>
    </div>
    <p class="small muted hide-phone">Bars run from the start (or creation) date to the due date; epics to their target date. Drag a bar's right end to change its date. Items without a due date are not shown.</p>
  {/if}
</div>

<style>
  .tl { display: flex; overflow: hidden; }
  .labels { width: 280px; flex: none; border-right: 1px solid var(--line); }
  .chart { flex: 1; overflow-x: auto; }
  .lhead, .thead { height: 40px; border-bottom: 1px solid var(--line); background: var(--panel); position: relative; }
  .lhead { padding: 12px 10px 0; font-weight: 600; }
  .lgroup, .cgroup { height: 26px; background: var(--panel); border-bottom: 1px solid var(--line2); }
  .lgroup { padding: 5px 10px; font-weight: 600; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
  .lrow, .crow { height: 26px; border-bottom: 1px solid var(--line2); position: relative; }
  .lrow { padding: 4px 10px 4px 18px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
  .tick { position: absolute; bottom: 3px; font-size: 10.5px; color: var(--muted); padding-left: 3px; white-space: nowrap; }
  .grid { position: absolute; top: 40px; bottom: 0; width: 1px; background: var(--line2); }
  .today { position: absolute; top: 0; bottom: 0; width: 2px; background: var(--err); z-index: 2; opacity: 0.7; }
  .sprint { position: absolute; top: 2px; height: 16px; background: var(--accent-soft); border: 1px solid var(--on-line); color: var(--on-text); font-size: 10.5px; padding: 0 4px; overflow: hidden; white-space: nowrap; border-radius: 3px; }
  .ms { position: absolute; top: 24px; width: 12px; height: 12px; background: #e0a100; transform: rotate(45deg); z-index: 3; border: 1px solid #8a6d1d; }
  .bar { position: absolute; top: 6px; height: 14px; border-radius: 3px; opacity: 0.85; }
  .handle { position: absolute; right: -3px; top: -2px; width: 8px; height: 18px; cursor: ew-resize; border-radius: 2px; background: rgba(255, 255, 255, 0.5); border: 1px solid rgba(0, 0, 0, 0.25); touch-action: none; }
  .deps { position: absolute; left: 0; top: 0; pointer-events: none; z-index: 1; overflow: visible; }
</style>
