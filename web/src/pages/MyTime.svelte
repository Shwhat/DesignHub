<script lang="ts">
  import { get, post } from '../lib/api';
  import { toast } from '../lib/state.svelte';
  import { today, addDays, hours, date } from '../lib/format';
  import RecordPicker from '../components/RecordPicker.svelte';

  function monday(s: string) {
    const [y, m, d] = s.split('-').map(Number);
    const dt = new Date(y, m - 1, d);
    const shift = (dt.getDay() + 6) % 7;
    return addDays(s, -shift);
  }
  let week = $state(monday(today()));
  let entries: any[] = $state([]);
  let form = $state({ item: '', day: today(), hours: '', note: '' });
  const days = $derived(Array.from({ length: 7 }, (_, i) => addDays(week, i)));

  async function load() {
    try { entries = await get(`/api/me/time?from=${week}&to=${addDays(week, 6)}`); } catch (e: any) { toast(e.message, 'error'); }
  }
  $effect(() => { week; load(); });
  async function add() {
    try {
      await post('/api/time', { item: form.item, day: form.day, hours: Number(form.hours), note: form.note });
      form = { ...form, hours: '', note: '' };
      load();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  const items = $derived.by(() => {
    const m: Record<string, { id: string; title: string; byDay: Record<string, number>; total: number }> = {};
    for (const e of entries) {
      const it = (m[e.itemId] ||= { id: e.itemId, title: e.title, byDay: {}, total: 0 });
      it.byDay[e.day] = (it.byDay[e.day] || 0) + e.hours;
      it.total += e.hours;
    }
    return Object.values(m);
  });
  const dayTotal = (d: string) => entries.filter((e) => e.day === d).reduce((a, e) => a + e.hours, 0);
  const weekTotal = $derived(entries.reduce((a, e) => a + e.hours, 0));
  const short = (d: string) => new Date(d + 'T00:00:00').toLocaleDateString(undefined, { weekday: 'short', day: 'numeric' });
</script>

<div class="content">
  <div class="row" style="margin-bottom: 10px">
    <h1 style="margin: 0">My time</h1><span class="spacer"></span>
    <button class="small" onclick={() => (week = addDays(week, -7))}>Previous week</button>
    <span><strong>{date(week)}</strong> to {date(addDays(week, 6))}</span>
    <button class="small" onclick={() => (week = addDays(week, 7))} disabled={addDays(week, 7) > today()}>Next week</button>
  </div>
  <div class="card card-body row" style="align-items: flex-end; margin-bottom: 12px">
    <div style="flex: 1; min-width: 220px"><label for="m-i">Work item</label>{#if form.item}<div class="row"><span class="id">{form.item}</span><button class="small ghost" onclick={() => (form.item = '')}>Change</button></div>{:else}<RecordPicker kinds="task,issue,action,ncr" onpick={(id) => (form.item = id)} />{/if}</div>
    <div style="width: 150px"><label for="m-d">Date</label><input id="m-d" type="date" max={today()} bind:value={form.day} /></div>
    <div style="width: 90px"><label for="m-h">Hours</label><input id="m-h" type="number" min="0.25" max="24" step="0.25" bind:value={form.hours} /></div>
    <div style="flex: 1; min-width: 140px"><label for="m-n">Note</label><input id="m-n" bind:value={form.note} /></div>
    <button class="primary" disabled={!form.item || !form.hours} onclick={add}>Log time</button>
  </div>
  <div class="card table-wrap">
    <table class="data">
      <thead><tr><th>Item</th>{#each days as d}<th class="num">{short(d)}</th>{/each}<th class="num">Total</th></tr></thead>
      <tbody>
        {#each items as it}<tr><td><a class="id" href="/work/{it.id}">{it.id}</a> {it.title}</td>{#each days as d}<td class="num">{it.byDay[d] || ''}</td>{/each}<td class="num"><strong>{it.total}</strong></td></tr>
        {:else}<tr><td colspan="9" class="empty">No time logged this week.</td></tr>{/each}
        {#if items.length}<tr style="font-weight: 600"><td>Total</td>{#each days as d}<td class="num">{dayTotal(d) || ''}</td>{/each}<td class="num">{hours(weekTotal)}</td></tr>{/if}
      </tbody>
    </table>
  </div>
  <p class="small muted">To change or delete an entry, open the item's Time tab.</p>
</div>
