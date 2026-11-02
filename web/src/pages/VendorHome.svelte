<script lang="ts">
  import { get } from '../lib/api';
  import { session } from '../lib/state.svelte';
  import { date } from '../lib/format';
  import Status from '../components/Status.svelte';

  // Home for Vendor accounts: their NCRs, actions and supplied items.
  let ncrs: any[] = $state([]);
  let actions: any[] = $state([]);
  let items: any[] = $state([]);
  get('/api/work?type=ncr&open=0').then((r) => (ncrs = r)).catch(() => {});
  get('/api/my-work').then((r) => (actions = r)).catch(() => {});
  get('/api/library').then((r) => (items = [...r].sort((a, b) => String(b.updated).localeCompare(String(a.updated))).slice(0, 10))).catch(() => {});
  const needing = $derived(ncrs.filter((n) => n.category !== 'done' && n.category !== 'cancelled'));
</script>

<div class="content">
  <h1>Welcome, {session.me?.displayName}</h1>
  <p class="muted">{session.me?.companyName}</p>
  <div class="grid2">
    <div class="card">
      <div class="card-head">Nonconformances needing a response<span class="spacer"></span><a class="small" href="/supplier-ncrs">All NCRs</a></div>
      {#each needing as n}<a class="comment row" href="/work/{n.id}"><span class="id">{n.id}</span><span style="flex: 1">{n.title}</span><Status value={n.status} /></a>
      {:else}<div class="empty">Nothing needs your response.</div>{/each}
    </div>
    <div class="card">
      <div class="card-head">Actions assigned to you</div>
      {#each actions as a}<a class="comment row" href="/work/{a.id}"><span class="id">{a.id}</span><span style="flex: 1">{a.title}</span>{#if a.dueDate}<span class="small muted">Due {date(a.dueDate)}</span>{/if}</a>
      {:else}<div class="empty">No actions.</div>{/each}
    </div>
    <div class="card">
      <div class="card-head">Recently changed supplied items<span class="spacer"></span><a class="small" href="/library">All items</a></div>
      {#each items as i}<a class="comment row" href="/items/{encodeURIComponent(i.id)}"><span class="id">{i.id}</span><span style="flex: 1">{i.title}</span><span class="small">rev {i.revision}</span></a>
      {:else}<div class="empty">No supplied items.</div>{/each}
    </div>
  </div>
</div>
