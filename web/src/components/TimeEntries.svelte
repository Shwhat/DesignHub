<script lang="ts">
  import { get, post, patch, del } from '../lib/api';
  import { toast } from '../lib/state.svelte';
  import { date, hours, today } from '../lib/format';

  let { itemId, onchange }: { itemId: string; onchange?: () => void } = $props();
  let d: any = $state(null);
  let form = $state({ day: today(), hours: '', note: '' });
  let editId = $state('');
  let edit = $state({ day: '', hours: '', note: '' });

  async function load() {
    d = await get(`/api/work/${itemId}/time`);
  }
  load();
  async function add() {
    try {
      await post('/api/time', { item: itemId, day: form.day, hours: Number(form.hours), note: form.note });
      form = { day: today(), hours: '', note: '' };
      toast('Time logged.');
      await load();
      onchange?.();
    } catch (e: any) {
      toast(e.message, 'error');
    }
  }
  async function saveEdit() {
    try {
      await patch('/api/time/' + editId, { day: edit.day, hours: Number(edit.hours), note: edit.note });
      editId = '';
      await load();
      onchange?.();
    } catch (e: any) {
      toast(e.message, 'error');
    }
  }
  async function remove(id: string) {
    if (!confirm('Delete this time entry?')) return;
    try {
      await del('/api/time/' + id);
      await load();
      onchange?.();
    } catch (e: any) {
      toast(e.message, 'error');
    }
  }
</script>

{#if d}
  <div class="card">
    <div class="card-head">
      <span>Time</span><span class="spacer"></span>
      <span class="muted small">{d.estimate !== null ? hours(d.estimate) + ' estimated, ' : ''}{hours(d.logged)} logged{d.estimate !== null ? ', ' + hours(Math.max(0, d.estimate - d.logged)) + ' remaining' : ''}</span>
    </div>
    {#if d.canLog}
      <div class="card-body row" style="align-items: flex-end; border-bottom: 1px solid var(--line2)">
        <div style="width: 150px"><label for="t-day">Date</label><input id="t-day" type="date" max={today()} bind:value={form.day} /></div>
        <div style="width: 100px"><label for="t-h">Hours</label><input id="t-h" type="number" min="0.25" max="24" step="0.25" bind:value={form.hours} /></div>
        <div style="flex: 1; min-width: 160px"><label for="t-n">Note</label><input id="t-n" bind:value={form.note} onkeydown={(e) => e.key === 'Enter' && add()} /></div>
        <button class="primary" disabled={!form.hours} onclick={add}>Log time</button>
      </div>
    {/if}
    <div class="table-wrap">
      <table class="data">
        <thead><tr><th>Date</th><th>Person</th><th class="num">Hours</th><th>Note</th><th></th></tr></thead>
        <tbody>
          {#each d.entries as t (t.id)}
            {#if editId === t.id}
              <tr>
                <td><input type="date" max={today()} bind:value={edit.day} /></td><td>{t.user}</td>
                <td><input type="number" min="0.25" max="24" step="0.25" bind:value={edit.hours} style="width: 80px" /></td>
                <td><input bind:value={edit.note} /></td>
                <td style="white-space: nowrap"><button class="small primary" onclick={saveEdit}>Save</button> <button class="small" onclick={() => (editId = '')}>Cancel</button></td>
              </tr>
            {:else}
              <tr>
                <td style="white-space: nowrap">{date(t.day)}</td><td>{t.user}</td><td class="num">{t.hours}</td><td>{t.note}{#if t.editedAt} <span class="muted small">(edited)</span>{/if}</td>
                <td style="white-space: nowrap">{#if t.canEdit}<button class="small ghost" onclick={() => { editId = t.id; edit = { day: t.day, hours: String(t.hours), note: t.note }; }}>Edit</button><button class="small ghost danger" onclick={() => remove(t.id)}>Delete</button>{/if}</td>
              </tr>
            {/if}
          {:else}
            <tr><td colspan="5" class="empty">No time logged.</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
{/if}
