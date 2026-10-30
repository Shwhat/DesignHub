<script lang="ts">
  import { get } from '../lib/api';
  import { parseArgs, blockLabels } from '../lib/markdown';
  import { recordPath } from '../lib/router.svelte';
  import { date, kindLabel } from '../lib/format';
  import BOMView from './BOMView.svelte';
  import Status from './Status.svelte';

  let { kind, args }: { kind: string; args: string } = $props();
  const a = $derived(parseArgs(args));
  let data: any = $state(null);
  let error = $state('');

  $effect(() => {
    kind; args;
    data = null;
    error = '';
    const { id, params } = parseArgs(args);
    let url = '';
    if (kind === 'work') {
      const q = new URLSearchParams();
      if (params.type) q.set('type', params.type);
      if (params.item) q.set('item', params.item);
      if (params.space) q.set('space', params.space);
      if (params.status) q.set('status', params.status);
      if (params.open === 'false' || params.open === '0') q.set('open', '0');
      url = '/api/work?' + q;
    } else if (kind === 'item') url = '/api/resolve?ids=' + encodeURIComponent(id);
    else if (kind === 'traceability') url = '/api/traceability' + (id ? '?item=' + encodeURIComponent(id) : '');
    else if (kind === 'changeorders') url = '/api/change-orders?filter=' + (params.filter || 'open') + (params.space ? '&space=' + params.space : '');
    if (!url) return;
    get(url).then((d) => (data = d)).catch((e) => (error = e.message));
  });
</script>

<div class="card card-body">
  <div class="muted small" style="margin-bottom: 0.4rem">{blockLabels[kind] || kind}</div>
  {#if kind === 'bom'}
    {#if a.id}<BOMView id={a.id} compact />{:else}<div class="muted">Choose an assembly.</div>{/if}
  {:else if error}
    <div class="error-box">{error}</div>
  {:else if !data}
    <div class="muted small">Loading...</div>
  {:else if kind === 'work'}
    {#if data.length === 0}<div class="muted small">No matching work items.</div>{/if}
    <div class="table-wrap"><table class="data"><tbody>
      {#each data as w}<tr><td><a class="id" href="/work/{w.id}">{w.id}</a></td><td>{w.title}</td><td><Status value={w.status} /></td><td class="hide-phone">{w.assignee}</td><td class="hide-phone">{date(w.dueDate)}</td></tr>{/each}
    </tbody></table></div>
  {:else if kind === 'item'}
    {@const r = data[a.id]}
    {#if r}<div class="row"><a class="id" href={recordPath(r.id, r.kind)}>{r.id}</a> {#if r.revision}<span class="muted">rev {r.revision}</span>{/if} <strong>{r.title}</strong> <Status value={r.status} /> <span class="muted small">{kindLabel(r.kind)}</span></div>
    {:else}<div class="muted">{a.id} is not available.</div>{/if}
  {:else if kind === 'traceability'}
    <div class="table-wrap"><table class="data">
      <thead><tr><th>Requirement</th><th>Satisfied by</th><th>Verified by</th><th>Coverage</th></tr></thead>
      <tbody>{#each data as t}<tr><td><a class="id" href="/items/{t.id}">{t.id}</a> {t.title}</td><td>{#each t.satisfiedBy as s}<a class="id" href="/items/{encodeURIComponent(s.id)}">{s.id}</a> {/each}</td><td>{#each t.verifiedBy as s}<a class="id" href="/items/{s.id}">{s.id}</a> {/each}</td><td><Status value={t.coverage === 'Complete' ? 'Complete' : t.coverage} /></td></tr>{/each}</tbody>
    </table></div>
  {:else if kind === 'changeorders'}
    {#if data.length === 0}<div class="muted small">No change orders.</div>{/if}
    <div class="table-wrap"><table class="data"><tbody>
      {#each data as c}<tr><td><a class="id" href="/change-orders/{c.id}">{c.id}</a></td><td>{c.title}</td><td><Status value={c.state} /></td><td class="hide-phone">{c.owner}</td></tr>{/each}
    </tbody></table></div>
  {/if}
</div>
