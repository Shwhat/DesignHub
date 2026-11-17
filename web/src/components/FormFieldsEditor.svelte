<script lang="ts">
  import Icon from './Icon.svelte';
  import Self from './FormFieldsEditor.svelte';
  import { fieldTypes, columnTypes, keyFrom, recordKinds, type FormField } from '../lib/forms';
  import { kindLabel } from '../lib/format';

  // Field list of a template form, or the columns of a table field.
  let { fields = $bindable(), columns = false, disabled = false, oninsert }:
    { fields: FormField[]; columns?: boolean; disabled?: boolean; oninsert?: (token: string) => void } = $props();

  const types = $derived(columns ? columnTypes : fieldTypes);
  let openKey: number | null = $state(null);
  let keyEdited: Record<number, boolean> = $state({});

  function add() {
    const n = fields.length + 1;
    fields = [...fields, { key: columns ? `column-${n}` : `field-${n}`, label: '', type: 'text', _new: true } as FormField];
    openKey = fields.length - 1;
  }
  function remove(i: number) {
    if (!confirm(`Remove ${fields[i].label || 'this field'}?`)) return;
    fields = fields.filter((_, j) => j !== i);
    openKey = null;
  }
  function move(i: number, d: number) {
    const j = i + d;
    if (j < 0 || j >= fields.length) return;
    const f = [...fields];
    [f[i], f[j]] = [f[j], f[i]];
    fields = f;
    openKey = openKey === i ? j : openKey;
  }
  function label(i: number, v: string) {
    fields[i].label = v;
    // Only new fields take their key from the label; the layout uses the keys
    // of existing ones.
    if ((fields[i] as any)._new && !keyEdited[i]) fields[i].key = keyFrom(v);
  }
  function setType(i: number, t: string) {
    const f = fields[i];
    f.type = t;
    if (t === 'select' && !f.options?.length) f.options = ['', ''];
    if (t === 'table' && !f.columns?.length) f.columns = [{ key: 'column-1', label: 'Column 1', type: 'text' }];
    if (t !== 'table') delete f.columns;
    if (t !== 'select') delete f.options;
    if (t !== 'record') delete f.kinds;
    openKey = i;
  }
  const kinds = (f: FormField) => (f.kinds || '').split(',').filter(Boolean);
  function toggleKind(f: FormField, k: string) {
    const set = new Set(kinds(f));
    if (set.has(k)) set.delete(k); else set.add(k);
    f.kinds = recordKinds.filter((x) => set.has(x)).join(',');
  }
  const typeLabel = (t: string) => types.find((x) => x.value === t)?.label || t;
</script>

<div class="ffe" class:cols={columns}>
  {#each fields as f, i}
    <div class="ffe-row">
      <div class="row ffe-main">
        <input aria-label="Label" placeholder={columns ? 'Column label' : 'Field label'} value={f.label} oninput={(e) => label(i, (e.target as HTMLInputElement).value)} {disabled} />
        <select aria-label="Type" value={f.type} onchange={(e) => setType(i, (e.target as HTMLSelectElement).value)} {disabled}>
          {#each types as t}<option value={t.value}>{t.label}</option>{/each}
        </select>
        {#if f.type !== 'index'}<label class="checkbox"><input type="checkbox" bind:checked={f.required} {disabled} /> Required</label>{/if}
        <span class="spacer"></span>
        {#if !columns && oninsert && !disabled}<button class="small" title="Add {'{{'}{f.key}{'}}'} to the end of the body" onclick={() => oninsert(`{{${f.key}}}`)}>Insert</button>{/if}
        <button class="ghost icon" aria-label="Settings" aria-expanded={openKey === i} onclick={() => (openKey = openKey === i ? null : i)}><Icon name={openKey === i ? 'down' : 'right'} size={14} /></button>
        {#if !disabled}
          <button class="ghost icon" aria-label="Move up" disabled={i === 0} onclick={() => move(i, -1)}><Icon name="up" size={14} /></button>
          <button class="ghost icon" aria-label="Move down" disabled={i === fields.length - 1} onclick={() => move(i, 1)}><Icon name="dn" size={14} /></button>
          <button class="ghost icon" aria-label="Remove" onclick={() => remove(i)}><Icon name="remove" size={14} /></button>
        {/if}
      </div>
      {#if openKey !== i}
        <div class="small muted ffe-sum"><span class="id">{'{{'}{f.key}{'}}'}</span> {typeLabel(f.type)}{f.options?.length ? ': ' + f.options.filter(Boolean).join(', ') : ''}{f.columns?.length ? ': ' + f.columns.map((c) => c.label).join(', ') : ''}</div>
      {:else}
        <div class="ffe-more">
          <div class="grid2">
            <div class="field"><label for="k{i}">Key</label><input id="k{i}" class="id" bind:value={f.key} oninput={() => (keyEdited[i] = true)} {disabled} />
              <div class="hint">{columns ? 'Lowercase letters, digits and hyphens.' : 'Put ' + '{{' + f.key + '}}' + ' in the body where this field goes.'}</div></div>
            {#if f.type !== 'index' && f.type !== 'table'}
              <div class="field"><label for="d{i}">Default</label><input id="d{i}" bind:value={f.default} placeholder={f.type === 'date' ? '{date}' : f.type === 'user' || f.type === 'users' ? '{user}' : ''} {disabled} />
                <div class="hint">{'{date}'}, {'{user}'} and {'{space}'} are filled in when the page is created.</div></div>
            {/if}
          </div>
          {#if f.type === 'select'}
            <div class="field"><label for="o{i}">Choices, one per line</label>
              <textarea id="o{i}" rows="4" value={(f.options || []).join('\n')} oninput={(e) => (f.options = (e.target as HTMLTextAreaElement).value.split('\n'))} {disabled}></textarea></div>
          {/if}
          {#if f.type === 'record'}
            <div class="field"><span class="lbl">Record types offered (none ticked: all)</span>
              <div class="row kinds">{#each recordKinds as k}<label class="checkbox"><input type="checkbox" checked={kinds(f).includes(k)} onchange={() => toggleKind(f, k)} {disabled} /> {kindLabel(k)}</label>{/each}</div></div>
          {/if}
          {#if !columns}
            <div class="field"><label for="h{i}">Help text</label><input id="h{i}" bind:value={f.help} placeholder="Shown under the field" {disabled} /></div>
          {/if}
          {#if f.type === 'table'}
            <span class="lbl">Columns</span>
            <Self bind:fields={f.columns!} columns {disabled} />
          {/if}
        </div>
      {/if}
    </div>
  {:else}
    <div class="muted small">{columns ? 'No columns.' : 'No fields. Pages from this template are written as text.'}</div>
  {/each}
  {#if !disabled}<button class="small" onclick={add}><Icon name="plus" size={14} />{columns ? 'Add column' : 'Add field'}</button>{/if}
</div>

<style>
  .ffe { display: grid; gap: 6px; justify-items: stretch; }
  .ffe > button { justify-self: start; }
  .ffe-row { border: 1px solid var(--line2); border-radius: 4px; padding: 6px 8px; background: var(--panel2); }
  .cols .ffe-row { background: var(--panel); }
  .ffe-main { flex-wrap: wrap; gap: 6px; }
  .ffe-main > input { flex: 1 1 180px; }
  .ffe-main > select { flex: 0 0 auto; width: auto; }
  .ffe-sum { margin-top: 3px; }
  .ffe-more { margin-top: 8px; }
  .kinds { flex-wrap: wrap; gap: 4px 12px; }
  .lbl { font-size: 11.5px; font-weight: 600; color: var(--text-2); display: block; margin-bottom: 3px; }
</style>
