<script lang="ts">
  import { get, patch } from '../lib/api';
  import { canIn, toast, onEvent, session, setSpace } from '../lib/state.svelte';
  import { recordPath } from '../lib/router.svelte';
  import { ago, kindLabel } from '../lib/format';
  import Status from '../components/Status.svelte';
  import Modal from '../components/Modal.svelte';
  import RecordPicker from '../components/RecordPicker.svelte';
  import { onDestroy } from 'svelte';
  let { params }: { params: any; pattern?: string } = $props();
  const id = params.id;
  let s: any = $state(null);
  let error = $state('');
  let pinOpen = $state(false);
  let pinTitle = $state('');
  let pinUrl = $state('');
  async function load() { try { s = await get('/api/spaces/' + id); setSpace(id); } catch (e: any) { error = e.message; } }
  load();
  const off = onEvent((e) => { if (e.space === id) load(); });
  onDestroy(() => off());
  async function savePins(links: any[]) {
    try { await patch('/api/spaces/' + id, { pinnedLinks: links }); load(); } catch (e: any) { toast(e.message, 'error'); }
  }
  const workLabels: Record<string, string> = { task: 'Tasks', issue: 'Issues', action: 'Actions', epic: 'Epics', ncr: 'NCRs' };
</script>

{#if error}<div class="content narrow"><div class="card card-body empty"><h2>Not found</h2><p>This space does not exist, or you are not a member.</p></div></div>
{:else if s}
<div class="content">
  <div class="record-head"><div class="titles"><div class="meta"><span class="id">{s.id}</span>{#if s.archived}<span class="status Cancelled">Archived</span>{/if}</div><h1>{s.name}</h1>
    {#if s.description}<p class="muted" style="margin: 0.3rem 0 0">{s.description}</p>{/if}</div>
    <div class="row"><a class="btn" href="/pages/{s.homePage}">Home page</a>{#if canIn(id, 'work.edit')}<a class="btn primary" href="/work/new?space={id}">New work item</a>{/if}</div>
  </div>
  <div class="grid2" style="align-items: start">
    <section class="card">
      <div class="card-head"><h2>Pinned links</h2><span class="spacer"></span>{#if canIn(id, 'space.pin')}<button class="small" onclick={() => (pinOpen = true)}>Pin a link</button>{/if}</div>
      {#each s.pinnedLinks as l, i}<div class="comment row"><a href={l.url.startsWith('/') ? l.url : recordPath(l.url)}>{l.title}</a><span class="spacer"></span>{#if canIn(id, 'space.pin')}<button class="small ghost" onclick={() => savePins(s.pinnedLinks.filter((_: any, j: number) => j !== i))}>Unpin</button>{/if}</div>
      {:else}<div class="empty">No pinned links.</div>{/each}
    </section>
    <section class="card">
      <div class="card-head"><h2>Open work</h2></div>
      <div class="card-body row">
        {#each Object.entries(workLabels) as [k, l]}<a class="btn" href="/spaces/{id}/{k === 'ncr' ? 'ncr-list' : 'list'}?type={k}">{l}: {s.workCounts[k] || 0}</a>{/each}
      </div>
    </section>
    <section class="card">
      <div class="card-head"><h2>Open change orders</h2></div>
      {#each s.openChangeOrders as c}<div class="comment row"><a class="id" href="/change-orders/{c.id}">{c.id}</a> {c.title}<span class="spacer"></span><Status value={c.state} /></div>
      {:else}<div class="empty">No open change orders.</div>{/each}
    </section>
    <section class="card">
      <div class="card-head"><h2>Recent changes</h2></div>
      {#each s.recent as r}<div class="comment row"><a class="id" href={recordPath(r.id, r.kind)}>{r.id}</a> <span style="flex: 1">{r.title}</span><span class="muted small">{kindLabel(r.kind)} {ago(r.updated)}</span></div>
      {:else}<div class="empty">Nothing yet.</div>{/each}
    </section>
  </div>
</div>
{#if pinOpen}
  <Modal title="Pin a link" onclose={() => (pinOpen = false)}>
    <div class="field"><label for="pt">Title</label><input id="pt" bind:value={pinTitle} /></div>
    <div class="field"><label for="pu">Record</label>{#if pinUrl}<div class="row"><span class="id">{pinUrl}</span><button class="small ghost" onclick={() => (pinUrl = '')}>Change</button></div>{:else}<RecordPicker onpick={(rid, r) => { pinUrl = rid; if (!pinTitle) pinTitle = r.title; }} />{/if}</div>
    {#snippet footer()}<button onclick={() => (pinOpen = false)}>Cancel</button><button class="primary" disabled={!pinTitle || !pinUrl} onclick={() => { savePins([...s.pinnedLinks, { title: pinTitle, url: pinUrl }]); pinOpen = false; pinTitle = ''; pinUrl = ''; }}>Pin</button>{/snippet}
  </Modal>
{/if}
{/if}
