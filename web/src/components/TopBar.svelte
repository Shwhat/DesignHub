<script lang="ts">
  import { get, post } from '../lib/api';
  import { session, can, setMe, disconnectEvents, refreshUnread } from '../lib/state.svelte';
  import { navigate, recordPath } from '../lib/router.svelte';
  import { kindLabel, ago } from '../lib/format';

  let q = $state('');
  let results: any[] = $state([]);
  let exact: any = $state(null);
  let focused = $state(false);
  let sel = $state(-1);
  let bellOpen = $state(false);
  let notes: any[] = $state([]);
  let profileOpen = $state(false);
  let timer: any;
  let ctrl: AbortController | null = null;
  let searchEl: HTMLElement;
  let bellEl: HTMLElement;
  let profEl: HTMLElement;
  let mobileSearch = $state(false);

  function search() {
    clearTimeout(timer);
    timer = setTimeout(async () => {
      if (!q.trim()) { results = []; exact = null; return; }
      ctrl?.abort();
      ctrl = new AbortController();
      try {
        const r = await get(`/api/search?limit=15&q=${encodeURIComponent(q)}`, { signal: ctrl.signal });
        results = r.results;
        exact = r.exact || null;
        sel = -1;
      } catch {}
    }, 160);
  }
  const groups = $derived.by(() => {
    const g: Record<string, any[]> = {};
    for (const r of results) (g[r.kind] ||= []).push(r);
    return Object.entries(g);
  });
  function go(r: any) {
    q = ''; results = []; focused = false; mobileSearch = false;
    navigate(recordPath(r.id, r.kind));
  }
  function key(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') { sel = Math.min(sel + 1, results.length - 1); e.preventDefault(); }
    else if (e.key === 'ArrowUp') { sel = Math.max(sel - 1, -1); e.preventDefault(); }
    else if (e.key === 'Escape') { focused = false; (e.target as HTMLElement).blur(); }
    else if (e.key === 'Enter') {
      e.preventDefault();
      if (sel >= 0 && results[sel]) go(results[sel]);
      else if (exact) go(exact);
      else if (q.trim()) {
        // An exact ID typed before results arrive still opens the record.
        get(`/api/search?limit=1&q=${encodeURIComponent(q)}`).then((r) => {
          if (r.exact) go(r.exact);
          else { const t = q; q = ''; results = []; focused = false; mobileSearch = false; navigate('/search?q=' + encodeURIComponent(t)); }
        });
      }
    }
  }
  async function openBell() {
    bellOpen = !bellOpen;
    if (bellOpen) {
      try { notes = (await get('/api/notifications?limit=8')).items; } catch {}
    }
  }
  async function openNote(n: any) {
    bellOpen = false;
    await post('/api/notifications/mark', { ids: [n.id], read: true }).catch(() => {});
    refreshUnread();
    navigate(recordPath(n.recordId, n.recordKind));
  }
  async function signOut() {
    try { await post('/api/signout'); } catch {}
    disconnectEvents();
    setMe(null);
    navigate('/signin');
  }
  function outside(e: MouseEvent) {
    const t = e.target as Node;
    if (focused && searchEl && !searchEl.contains(t)) focused = false;
    if (bellOpen && bellEl && !bellEl.contains(t)) bellOpen = false;
    if (profileOpen && profEl && !profEl.contains(t)) profileOpen = false;
  }
  const isAdmin = $derived(can('users.manage') || can('system.manage') || can('lists.manage') || can('audit.view'));
  let flat = $derived(results);
</script>

<svelte:window onclick={outside} />
<header class="topbar">
  <button class="ghost icon menu-btn" aria-label="Menu" onclick={() => (session.sidebarOpen = !session.sidebarOpen)}>☰</button>
  <a class="brand" href="/"><img src="/logo.svg" alt="" /><span class="hide-phone">DesignHub</span></a>
  <div class="search" class:mobile-open={mobileSearch} bind:this={searchEl}>
    <input type="search" placeholder="Search or type an ID" aria-label="Search" bind:value={q} oninput={search} onkeydown={key} onfocus={() => (focused = true)} />
    {#if focused && q.trim() && (results.length || exact)}
      <div class="quick card">
        {#each groups as [kind, rs]}
          <div class="nav-section">{kindLabel(kind)}</div>
          {#each rs as r}
            <button class="qr" class:sel={flat.indexOf(r) === sel} onclick={() => go(r)}><span class="id">{r.id}</span><span class="t">{r.title}</span></button>
          {/each}
        {/each}
        <a class="qr all" href="/search?q={encodeURIComponent(q)}" onclick={() => { focused = false; mobileSearch = false; q = ''; }}>All results for "{q}"</a>
      </div>
    {/if}
  </div>
  <button class="ghost icon phone-only" aria-label="Search" onclick={() => (mobileSearch = !mobileSearch)}>
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
  </button>
  <span class="spacer"></span>
  <div class="menu-wrap" bind:this={bellEl}>
    <button class="ghost icon bell" aria-label="Notifications" onclick={openBell}>
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/></svg>
      {#if session.unread > 0}<span class="dot">{session.unread > 99 ? '99+' : session.unread}</span>{/if}
    </button>
    {#if bellOpen}
      <div class="menu notes">
        {#each notes as n}
          <button onclick={() => openNote(n)} class:unread={!n.read}>
            <span><strong>{n.event}</strong> <span class="muted small">{ago(n.createdAt)}</span><br /><span class="small">{n.summary}</span></span>
          </button>
        {:else}
          <div class="empty">No notifications.</div>
        {/each}
        <hr />
        <a href="/notifications" onclick={() => (bellOpen = false)}>View all</a>
      </div>
    {/if}
  </div>
  <div class="menu-wrap" bind:this={profEl}>
    <button class="ghost avatar" aria-label="Profile menu" onclick={() => (profileOpen = !profileOpen)}>{(session.me?.displayName || '?').slice(0, 1).toUpperCase()}</button>
    {#if profileOpen}
      <div class="menu">
        <div style="padding: 0.4rem 0.65rem"><strong>{session.me?.displayName}</strong><br /><span class="muted small">{session.me?.libraryRole}</span></div>
        <hr />
        <a href="/settings/profile" onclick={() => (profileOpen = false)}>My profile</a>
        <a href="/settings/appearance" onclick={() => (profileOpen = false)}>Settings</a>
        {#if isAdmin}<a href="/admin" onclick={() => (profileOpen = false)}>Admin</a>{/if}
        <hr />
        <button onclick={signOut}>Sign out</button>
      </div>
    {/if}
  </div>
</header>

<style>
  .menu-btn { display: none; }
  @media (max-width: 900px) { .menu-btn { display: inline-flex; } }
  .search { position: relative; flex: 1; max-width: 520px; margin-left: 1rem; }
  .search input { background: var(--surface-2); border-color: var(--border); }
  @media (max-width: 600px) {
    .search { display: none; position: fixed; top: var(--topbar); left: 0; right: 0; margin: 0; max-width: none; padding: 0.5rem; background: var(--surface); border-bottom: 1px solid var(--border); }
    .search.mobile-open { display: block; }
  }
  .quick { position: absolute; top: calc(100% + 4px); left: 0; right: 0; max-height: 70vh; overflow: auto; z-index: 60; padding: 0.3rem; }
  .qr { display: flex; gap: 0.6rem; width: 100%; justify-content: flex-start; border: 0; background: none; font-weight: 400; padding: 0.4rem 0.6rem; min-height: 32px; text-align: left; }
  .qr:hover, .qr.sel { background: var(--surface-2); }
  .qr .t { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .qr.all { color: var(--link); font-size: 0.88rem; }
  .bell { position: relative; }
  .dot { position: absolute; top: 2px; right: 0; background: var(--danger); color: #fff; font-size: 0.65rem; font-weight: 700; border-radius: 999px; padding: 0 0.3rem; min-width: 1rem; }
  .notes { width: min(360px, 92vw); max-height: 70vh; overflow: auto; }
  .notes button { text-align: left; white-space: normal; }
  .notes button.unread { background: var(--accent-soft); }
  .avatar { width: 36px; height: 36px; border-radius: 50%; background: var(--accent); color: var(--accent-text); font-weight: 700; padding: 0; }
  .avatar:hover { background: var(--accent-hover); }
</style>
