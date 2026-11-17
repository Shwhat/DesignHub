<script lang="ts">
  import Icon from './Icon.svelte';
  import RecordPicker from './RecordPicker.svelte';
  import { session } from '../lib/state.svelte';
  import { emptyRow, type FormField, type Missing } from '../lib/forms';

  // The fields of a page form. values is changed in place.
  let { fields, values = $bindable(), miss = [], showErrors = false, disabled = false }:
    { fields: FormField[]; values: Record<string, any>; miss?: Missing[]; showErrors?: boolean; disabled?: boolean } = $props();

  const uid = Math.random().toString(36).slice(2, 8);
  const people = $derived(session.users.filter((u) => u.active).map((u) => u.displayName));
  const bad = (key: string) => showErrors && miss.some((m) => m.key === key || m.key.startsWith(key + '.'));
  const badCell = (key: string, i: number, col: string) => showErrors && miss.some((m) => m.key === `${key}.${i}.${col}`);
  let adding: Record<string, string> = $state({});

  function addPerson(key: string) {
    const name = (adding[key] || '').trim();
    if (!name) return;
    if (!values[key].includes(name)) values[key] = [...values[key], name];
    adding[key] = '';
  }
  function addItem(key: string) { values[key] = [...values[key], '']; }
  function removeAt(key: string, i: number) { values[key] = values[key].filter((_: any, j: number) => j !== i); }
  function addRow(f: FormField) { values[f.key] = [...values[f.key], emptyRow(f)]; }
  function removeRow(f: FormField, i: number) {
    const rows = values[f.key].filter((_: any, j: number) => j !== i);
    values[f.key] = rows.length ? rows : [emptyRow(f)];
  }
  function moveRow(f: FormField, i: number, d: number) {
    const rows = [...values[f.key]];
    const j = i + d;
    if (j < 0 || j >= rows.length) return;
    [rows[i], rows[j]] = [rows[j], rows[i]];
    values[f.key] = rows;
  }
  function listKey(e: KeyboardEvent, key: string, i: number) {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (i === values[key].length - 1) addItem(key);
      setTimeout(() => ((e.target as HTMLElement).closest('.list')?.querySelectorAll('input')[i + 1] as HTMLInputElement | undefined)?.focus());
    }
  }
</script>

<datalist id="people-{uid}">{#each people as p}<option value={p}></option>{/each}</datalist>

<div class="form-fill">
  {#each fields as f (f.key)}
    {@const id = `ff-${uid}-${f.key}`}
    <div class="field" class:error={bad(f.key)} class:wide={f.type === 'table' || f.type === 'textarea' || f.type === 'list' || f.type === 'users'}>
      {#if f.type === 'toggle'}
        <label class="checkbox toggle"><input {id} type="checkbox" bind:checked={values[f.key]} {disabled} /> {f.label}{#if f.required}<span class="req" title="Required">*</span>{/if}</label>
      {:else}
        <label for={id}>{f.label}{#if f.required}<span class="req" title="Required">*</span>{/if}</label>
      {/if}

      {#if f.type === 'text'}
        <input {id} bind:value={values[f.key]} {disabled} />
      {:else if f.type === 'textarea'}
        <textarea {id} rows="4" bind:value={values[f.key]} {disabled}></textarea>
      {:else if f.type === 'date'}
        <input {id} type="date" bind:value={values[f.key]} {disabled} />
      {:else if f.type === 'user'}
        <input {id} list="people-{uid}" autocomplete="off" placeholder="Choose or type a name" bind:value={values[f.key]} {disabled} />
      {:else if f.type === 'select'}
        <select {id} bind:value={values[f.key]} {disabled}>
          <option value="">Choose...</option>
          {#each f.options || [] as o}<option value={o}>{o}</option>{/each}
        </select>
      {:else if f.type === 'record'}
        {#if values[f.key]?.id}
          <div class="row"><span class="chip"><span class="id">{values[f.key].id}</span>&nbsp;{values[f.key].title}{#if !disabled}<button aria-label="Remove" onclick={() => (values[f.key] = null)}><Icon name="remove" size={14} /></button>{/if}</span></div>
        {:else if !disabled}
          <RecordPicker kinds={f.kinds || ''} onpick={(rid, r) => (values[f.key] = { id: rid, title: r?.title || '' })} />
        {/if}
      {:else if f.type === 'users'}
        <div>{#each values[f.key] as p, i}<span class="chip">{p}{#if !disabled}<button aria-label="Remove {p}" onclick={() => removeAt(f.key, i)}><Icon name="remove" size={14} /></button>{/if}</span>{/each}</div>
        {#if !disabled}
          <div class="row">
            <input {id} list="people-{uid}" autocomplete="off" placeholder="Choose or type a name, then Add" bind:value={adding[f.key]}
              onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addPerson(f.key); } }} onchange={() => { if (people.includes((adding[f.key] || '').trim())) addPerson(f.key); }} />
            <button onclick={() => addPerson(f.key)}>Add</button>
          </div>
        {/if}
      {:else if f.type === 'list'}
        <div class="list">
          {#each values[f.key] as _, i}
            <div class="row"><span class="muted small bullet">{i + 1}.</span><input id={i === 0 ? id : undefined} aria-label="{f.label} {i + 1}" bind:value={values[f.key][i]} onkeydown={(e) => listKey(e, f.key, i)} {disabled} />{#if !disabled}<button class="ghost icon" aria-label="Remove" onclick={() => removeAt(f.key, i)}><Icon name="remove" size={14} /></button>{/if}</div>
          {/each}
          {#if !disabled}<button class="small" onclick={() => addItem(f.key)}><Icon name="plus" size={14} />Add item</button>{/if}
        </div>
      {:else if f.type === 'table'}
        <div class="table-wrap"><table class="data form-table">
          <thead><tr>{#each f.columns || [] as c}<th class:narrow={c.type === 'index' || c.type === 'toggle'}>{c.label}{#if c.required}<span class="req">*</span>{/if}</th>{/each}{#if !disabled}<th class="narrow"></th>{/if}</tr></thead>
          <tbody>
            {#each values[f.key] as row, i}
              <tr>
                {#each f.columns || [] as c}
                  <td class:error={badCell(f.key, i, c.key)} class:narrow={c.type === 'index' || c.type === 'toggle'}>
                    {#if c.type === 'index'}<span class="muted">{i + 1}</span>
                    {:else if c.type === 'text'}<input aria-label="{c.label} row {i + 1}" bind:value={row[c.key]} {disabled} />
                    {:else if c.type === 'date'}<input type="date" aria-label="{c.label} row {i + 1}" bind:value={row[c.key]} {disabled} />
                    {:else if c.type === 'user'}<input list="people-{uid}" autocomplete="off" aria-label="{c.label} row {i + 1}" bind:value={row[c.key]} {disabled} />
                    {:else if c.type === 'select'}<select aria-label="{c.label} row {i + 1}" bind:value={row[c.key]} {disabled}><option value=""></option>{#each c.options || [] as o}<option value={o}>{o}</option>{/each}</select>
                    {:else if c.type === 'toggle'}<input type="checkbox" aria-label="{c.label} row {i + 1}" bind:checked={row[c.key]} {disabled} />
                    {:else if c.type === 'record'}
                      {#if row[c.key]?.id}<span class="chip"><span class="id">{row[c.key].id}</span>&nbsp;{row[c.key].title}{#if !disabled}<button aria-label="Remove" onclick={() => (row[c.key] = null)}><Icon name="remove" size={14} /></button>{/if}</span>
                      {:else if !disabled}<RecordPicker kinds={c.kinds || ''} placeholder="Type an ID or title" onpick={(rid, r) => (row[c.key] = { id: rid, title: r?.title || '' })} />{/if}
                    {/if}
                  </td>
                {/each}
                {#if !disabled}
                  <td class="narrow actions">
                    <button class="ghost icon" aria-label="Move up" disabled={i === 0} onclick={() => moveRow(f, i, -1)}><Icon name="up" size={14} /></button>
                    <button class="ghost icon" aria-label="Move down" disabled={i === values[f.key].length - 1} onclick={() => moveRow(f, i, 1)}><Icon name="dn" size={14} /></button>
                    <button class="ghost icon" aria-label="Remove row" onclick={() => removeRow(f, i)}><Icon name="remove" size={14} /></button>
                  </td>
                {/if}
              </tr>
            {/each}
          </tbody>
        </table></div>
        {#if !disabled}<button class="small" style="margin-top: 4px" onclick={() => addRow(f)}><Icon name="plus" size={14} />Add row</button>{/if}
      {/if}
      {#if f.help}<div class="hint">{f.help}</div>{/if}
    </div>
  {/each}
</div>

<style>
  .form-fill { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: 12px; }
  .form-fill .wide { grid-column: 1 / -1; }
  @media (max-width: 600px) { .form-fill { grid-template-columns: 1fr; } }
  .req { color: var(--err); margin-left: 2px; }
  .toggle { min-height: 28px; font-weight: 600; }
  .list { display: grid; gap: 4px; justify-items: start; }
  .list .row { width: 100%; flex-wrap: nowrap; }
  .list input { flex: 1; }
  .bullet { width: 1.6em; text-align: right; flex: none; }
  .form-table td { padding: 3px 4px; vertical-align: top; }
  .form-table input:not([type='checkbox']), .form-table select { width: 100%; min-width: 7em; }
  .form-table .narrow { width: 1%; white-space: nowrap; }
  .form-table td.error input, .form-table td.error select { border-color: var(--err); }
  .actions { display: flex; gap: 0; }
</style>
