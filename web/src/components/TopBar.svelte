<script lang="ts">
  import { get, post } from '../lib/api';
  import { session, can, setMe, disconnectEvents, refreshUnread } from '../lib/state.svelte';
  import { navigate, recordPath } from '../lib/router.svelte';
  import { kindLabel, ago } from '../lib/format';
  import Icon from './Icon.svelte';

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
  <button class="ghost icon menu-btn" aria-label="Menu" onclick={() => (session.sidebarOpen = !session.sidebarOpen)}><Icon name="menu" size={20} /></button>
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
    <Icon name="search" size={20} />
  </button>
  <span class="spacer"></span>
  <div class="menu-wrap" bind:this={bellEl}>
    <button class="ghost icon bell" aria-label="Notifications" onclick={openBell}>
      <Icon name="bell" size={20} />
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
        <div style="padding: 4px 14px 6px"><strong>{session.me?.displayName}</strong><br /><span class="muted small">{session.me?.libraryRole}</span></div>
        <hr />
        <a href="/settings/profile" onclick={() => (profileOpen = false)}><Icon name="user" />My profile</a>
        <a href="/settings/appearance" onclick={() => (profileOpen = false)}><Icon name="settings" />Settings</a>
        {#if isAdmin}<a href="/admin" onclick={() => (profileOpen = false)}><Icon name="admin" />Admin</a>{/if}
        <a href="/about" onclick={() => (profileOpen = false)}><Icon name="about" />About DesignHub</a>
        <hr />
        <button onclick={signOut}><Icon name="signout" />Sign out</button>
      </div>
    {/if}
  </div>
</header>

<style>
  .menu-btn { display: none; }
  @media (max-width: 900px) { .menu-btn { display: inline-flex; } }
  .search { position: relative; flex: 1; max-width: 480px; margin-left: 12px; }
  .search input { background: var(--panel2); border-color: transparent; }
  .search input:focus { border-color: var(--accent); }
  @media (max-width: 600px) {
    .search { display: none; position: fixed; top: var(--topbar); left: 0; right: 0; margin: 0; max-width: none; padding: 6px 8px; background: var(--menubar); }
    .search.mobile-open { display: block; }
  }
  .quick { position: absolute; top: calc(100% + 2px); left: 0; right: 0; max-height: 70vh; overflow: auto; z-index: 60; padding: 4px 0; color: var(--text); border-color: var(--pop-line); box-shadow: var(--shadow-pop); border-radius: 0; }
  .quick .nav-section { margin-top: 2px; }
  .qr { display: flex; gap: 8px; width: 100%; justify-content: flex-start; border: 0; border-radius: 0; background: none; font-weight: 400; padding: 4px 14px; min-height: 26px; text-align: left; color: var(--text); }
  .qr:hover, .qr.sel { background: var(--accent); color: #fff; text-decoration: none; }
  .qr .t { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .qr.all { color: var(--accent); }
  .qr.all:hover { color: #fff; }
  .bell { position: relative; }
  .dot { position: absolute; top: 0; right: -2px; background: var(--attention); color: var(--attention-text); font-size: 10px; font-weight: 600; border-radius: 8px; padding: 0 4px; min-width: 15px; line-height: 15px; }
  .notes { width: min(360px, 92vw); max-height: 70vh; overflow: auto; }
  .notes button { white-space: normal; align-items: flex-start; }
  .notes button.unread { background: var(--accent-soft); }
  .notes button.unread:hover { background: var(--accent); }
  .avatar { width: 28px; height: 28px; min-height: 28px; border-radius: 50%; background: var(--accent); color: var(--accent-text); font-weight: 600; padding: 0; border: 0; }
  .avatar:hover { background: var(--accent-hover); }
</style>
