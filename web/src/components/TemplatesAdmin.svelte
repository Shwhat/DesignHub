<script lang="ts">
  import { get, post, patch } from '../lib/api';
  import { toast, session, can, canIn } from '../lib/state.svelte';
  import { dateTime } from '../lib/format';
  import { listOf, loadLists } from '../lib/lists.svelte';
  import Editor from './Editor.svelte';

  // Admin, Templates, and the same table for one space's Admins.
  let { space = '' }: { space?: string } = $props();
  loadLists();
  let rows: any[] = $state([]);
  let edit: any = $state(null);

  async function load() {
    const all = can('templates.manage') && !space ? await get('/api/templates') : await get(`/api/templates?space=${space}`);
    rows = all;
  }
  load();
  async function open(t: any) {
    const r = await get('/api/templates/' + t.id);
    edit = { ...r.template, canEdit: r.canEdit };
  }
  function create() {
    edit = { id: '', name: '', space, description: '', pageType: 'General', titlePattern: '', body: '# {title}\n\n', active: true, canEdit: true, version: 0 };
  }
  async function save() {
    try {
      if (edit.id) await patch('/api/templates/' + edit.id, { ...edit });
      else await post('/api/templates', { ...edit });
      toast('Template saved.');
      edit = null;
      load();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  async function copy(t: any) {
    const r = await get('/api/templates/' + t.id);
    edit = { ...r.template, id: '', name: r.template.name + ' (copy)', space, canEdit: true, version: 0 };
  }
  const spaces = $derived(session.me?.spaces || []);
  const spaceLabel = (id: string) => (id ? spaces.find((s) => s.id === id)?.name || id : 'All spaces');
  const canCreate = $derived(space ? can('templates.manage') || canIn(space, 'space.admin') : can('templates.manage'));
</script>

{#if edit}
  <div class="card card-body">
    <div class="row" style="margin-bottom: 8px"><h2 style="margin: 0">{edit.id ? edit.name : 'New template'}</h2><span class="spacer"></span><button onclick={() => (edit = null)}>Cancel</button>{#if edit.canEdit}<button class="primary" onclick={save}>Save</button>{/if}</div>
    <div class="grid2">
      <div class="field"><label for="t-n">Name</label><input id="t-n" maxlength="60" bind:value={edit.name} disabled={!edit.canEdit} /></div>
      <div class="field"><label for="t-s">Scope</label>
        <select id="t-s" bind:value={edit.space} disabled={!edit.canEdit || !!edit.id || !!space}>
          {#if can('templates.manage')}<option value="">All spaces</option>{/if}
          {#each spaces as s}<option value={s.id}>{s.name}</option>{/each}
        </select></div>
      <div class="field"><label for="t-p">Page type</label><select id="t-p" bind:value={edit.pageType} disabled={!edit.canEdit}>{#each listOf('page_types') as p}<option>{p}</option>{/each}</select></div>
      <div class="field"><label for="t-t">Title pattern</label><input id="t-t" bind:value={edit.titlePattern} placeholder={'Design review {date}'} disabled={!edit.canEdit} /></div>
    </div>
    <div class="field"><label for="t-d">Description (shown in the New page dialog)</label><input id="t-d" bind:value={edit.description} disabled={!edit.canEdit} /></div>
    <label class="checkbox" style="margin-bottom: 8px"><input type="checkbox" bind:checked={edit.active} disabled={!edit.canEdit} /> Active</label>
    <div class="row small" style="margin-bottom: 4px">
      <span class="muted">Placeholders, filled when a page is created:</span>
      {#each ['{date}', '{user}', '{space}', '{title}'] as p}<button class="small" disabled={!edit.canEdit} onclick={() => (edit.body += p)}>{p}</button>{/each}
    </div>
    {#if edit.canEdit}<Editor bind:value={edit.body} recordId="" />{:else}<pre class="markdown">{edit.body}</pre>{/if}
    <p class="small muted">Changing a template does not change pages already created from it.</p>
  </div>
{:else}
  <div class="row" style="margin-bottom: 8px"><span class="muted small">Templates offered in the New page dialog.</span><span class="spacer"></span>{#if canCreate}<button class="primary" onclick={create}>New template</button>{/if}</div>
  <div class="card table-wrap"><table class="data">
    <thead><tr><th>Name</th><th>Scope</th><th>Page type</th><th>Active</th><th>Changed</th><th></th></tr></thead>
    <tbody>
      {#each rows as t (t.id)}
        <tr class="clickable" onclick={() => open(t)}><td><strong>{t.name}</strong><div class="small muted">{t.description}</div></td><td>{spaceLabel(t.space)}</td><td>{t.pageType}</td><td>{t.active ? 'Yes' : 'No'}</td><td class="small">{t.changedBy}, {dateTime(t.changedAt)}</td>
          <td onclick={(e) => e.stopPropagation()}>{#if canCreate}<button class="small" onclick={() => copy(t)}>Copy...</button>{/if}</td></tr>
      {:else}<tr><td colspan="6" class="empty">No templates.</td></tr>{/each}
    </tbody>
  </table></div>
{/if}

