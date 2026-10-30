<script lang="ts">
  import { get } from '../lib/api';
  import { session, onEvent } from '../lib/state.svelte';
  import { recordPath } from '../lib/router.svelte';
  import { date, ago, kindLabel } from '../lib/format';
  import { HOME_SECTIONS, homeLayout } from '../lib/home';
  import Status from '../components/Status.svelte';
  import { onDestroy } from 'svelte';
  let { params }: { params?: any; pattern?: string } = $props();

  let data: any = $state(null);
  async function load() { try { data = await get('/api/home'); } catch {} }
  load();
  const off = onEvent((e) => { if (e.type === 'notification' || e.type === 'board') load(); });
  onDestroy(() => off());
  const layout = $derived(homeLayout(session.me?.homeLayout).filter((s) => s.on));
  const title = (k: string) => HOME_SECTIONS.find((s) => s.key === k)?.title || k;
</script>

<div class="content">
  <h1>Welcome, {session.me?.displayName}</h1>
  {#if !data}<div class="muted">Loading...</div>{:else}
  <div class="grid2" style="align-items: start">
    {#each layout as s (s.key)}
      <section class="card">
        <div class="card-head"><h2>{title(s.key)}</h2></div>
        {#if s.key === 'assigned'}
          {#each data.assigned.slice(0, 10) as w}
            <a class="line" href="/work/{w.id}"><span class="id">{w.id}</span><span class="t">{w.title}</span><Status value={w.status} />{#if w.dueDate}<span class="small" class:over={w.overdue}>{date(w.dueDate)}</span>{/if}</a>
          {:else}<div class="empty">Nothing assigned to you.</div>{/each}
          {#if data.assigned.length > 10}<a class="line more" href="/my-work">All {data.assigned.length} items</a>{/if}
        {:else if s.key === 'approvals'}
          {#each data.approvals as c}<a class="line" href="/change-orders/{c.id}"><span class="id">{c.id}</span><span class="t">{c.title}</span><Status value={c.state} /></a>{/each}
          {#each data.dispositions as n}<a class="line" href="/work/{n.id}"><span class="id">{n.id}</span><span class="t">{n.title}</span><span class="pill">Disposition</span></a>{/each}
          {#each data.toRelease || [] as c}<a class="line" href="/change-orders/{c.id}"><span class="id">{c.id}</span><span class="t">{c.title}</span><span class="pill">Ready to release</span></a>{/each}
          {#if !data.approvals.length && !data.dispositions.length && !(data.toRelease || []).length}<div class="empty">Nothing awaiting your approval.</div>{/if}
        {:else if s.key === 'myItems'}
          {#each data.myItems as r}<a class="line" href={recordPath(r.id, r.kind)}><span class="id">{r.id}</span><span class="t">{r.title}</span><Status value={r.status.startsWith('Draft') ? 'Draft' : r.status} /></a>{:else}<div class="empty">No items in progress.</div>{/each}
        {:else if s.key === 'recent'}
          {#each data.recent as r}<a class="line" href={recordPath(r.id, r.kind)}><span class="id">{r.id}</span><span class="t">{r.title}</span><span class="muted small">{kindLabel(r.kind)}</span></a>{:else}<div class="empty">Nothing viewed yet.</div>{/each}
        {:else if s.key === 'activity'}
          {#each data.activity as r}<a class="line" href={recordPath(r.id, r.kind)}><span class="id">{r.id}</span><span class="t">{r.title}</span><span class="muted small">{ago(r.updated)}</span></a>{:else}<div class="empty">No recent activity.</div>{/each}
        {:else if s.key === 'notifications'}
          {#each data.notifications as n}<a class="line" href={recordPath(n.recordId, n.recordKind)}><span class="t"><strong>{n.event}</strong> {n.summary}</span><span class="muted small">{ago(n.createdAt)}</span></a>{:else}<div class="empty">No unread notifications.</div>{/each}
        {/if}
      </section>
    {/each}
  </div>
  {/if}
</div>

<style>
  .line { display: flex; align-items: center; gap: 0.6rem; padding: 0.55rem 1.15rem; border-bottom: 1px solid var(--border); color: var(--text); font-size: 0.9rem; }
  .line:last-child { border-bottom: 0; }
  .line:hover { background: var(--surface-2); text-decoration: none; }
  .line .t { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .line.more { color: var(--link); }
  .over { color: var(--danger); font-weight: 600; }
</style>
