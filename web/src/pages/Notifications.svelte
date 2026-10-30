<script lang="ts">
  import { get, post } from '../lib/api';
  import { refreshUnread, onEvent } from '../lib/state.svelte';
  import { navigate, recordPath } from '../lib/router.svelte';
  import { ago } from '../lib/format';
  import { onDestroy } from 'svelte';
  let { params }: { params?: any; pattern?: string } = $props();
  let filter = $state('all');
  let items: any[] = $state([]);
  async function load() { items = (await get('/api/notifications?filter=' + filter)).items; }
  $effect(() => { filter; load(); });
  const off = onEvent((e) => { if (e.type === 'notification') load(); });
  onDestroy(() => off());
  async function mark(body: any) { await post('/api/notifications/mark', body); refreshUnread(); load(); }
  async function open(n: any) { await mark({ ids: [n.id], read: true }); navigate(recordPath(n.recordId, n.recordKind)); }
</script>

<div class="content narrow">
  <div class="row" style="margin-bottom: 1rem"><h1 style="margin: 0">Notifications</h1><span class="spacer"></span><button onclick={() => mark({ all: true })}>Mark all read</button></div>
  <div class="tabs">{#each [['all', 'All'], ['unread', 'Unread'], ['mentions', 'Mentions'], ['approvals', 'Approvals']] as [k, l]}<button class:active={filter === k} onclick={() => (filter = k)}>{l}</button>{/each}</div>
  <div class="card">
    {#each items as n (n.id)}
      <div class="comment row" style={n.read ? '' : 'background: var(--accent-soft)'}>
        <button class="ghost" style="flex: 1; justify-content: flex-start; text-align: left; white-space: normal; height: auto" onclick={() => open(n)}>
          <span><strong>{n.event}</strong> <span class="id">{n.recordId}</span> {n.recordTitle}<br /><span class="small">{n.summary}</span><br /><span class="muted small">{n.by}, {ago(n.createdAt)}</span></span>
        </button>
        <button class="small ghost" onclick={() => mark({ ids: [n.id], read: !n.read })}>{n.read ? 'Mark unread' : 'Mark read'}</button>
        <button class="small ghost" onclick={() => mark({ ids: [n.id], dismiss: true })}>Dismiss</button>
      </div>
    {:else}<div class="empty">No notifications.</div>{/each}
  </div>
</div>
