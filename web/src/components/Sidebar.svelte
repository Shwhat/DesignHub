<script lang="ts">
  import { session, setSpace, can, canIn, onEvent, toast } from '../lib/state.svelte';
  import { route, navigate } from '../lib/router.svelte';
  import { get, post } from '../lib/api';
  import Modal from './Modal.svelte';
  import { onDestroy } from 'svelte';

  let tree: { home: string; pages: any[] } = $state({ home: '', pages: [] });
  let open: Record<string, boolean> = $state({});
  let menuFor = $state('');
  let moveFor: any = $state(null);
  let moveParent = $state('');
  let dragId = $state('');
  let dropTarget = $state({ id: '', where: '' });

  const spaces = $derived((session.me?.spaces || []).filter((s) => !s.archived));
  const space = $derived(session.space);

  async function loadTree() {
    if (!space) { tree = { home: '', pages: [] }; return; }
    try {
      tree = await get(`/api/spaces/${space}/tree`, { background: true });
      // Expand the path to the open page.
      const m = route.path.match(/^\/pages\/(PG-\d+)/);
      if (m) {
        let p = tree.pages.find((x) => x.id === m[1]);
        const o = { ...open };
        while (p && p.parent) { o[p.parent] = true; p = tree.pages.find((x) => x.id === p.parent); }
        open = o;
      }
    } catch {}
  }
  $effect(() => { space; loadTree(); });
  $effect(() => {
    // When a page of another space opens, switch the sidebar to it.
    const m = route.path.match(/^\/pages\/(PG-\d+)/);
    if (m && !tree.pages.some((p) => p.id === m[1])) {
      get(`/api/resolve?ids=${m[1]}`, { background: true }).then((r) => { const s = r[m[1]]?.space; if (s && s !== session.space) setSpace(s); else loadTree(); }).catch(() => {});
    }
    const sm = route.path.match(/^\/spaces\/(SP-\d+)/);
    if (sm && sm[1] !== session.space) setSpace(sm[1]);
  });
  const off = onEvent((e) => { if ((e.type === 'tree' || e.type === 'record') && e.space === space) loadTree(); });
  onDestroy(() => off());

  function children(parent: string) {
    const list = parent === tree.home ? tree.pages.filter((p) => p.parent === '' && p.id !== tree.home) : tree.pages.filter((p) => p.parent === parent);
    return list.sort((a, b) => a.sort - b.sort);
  }
  const hasChildren = (id: string) => children(id).length > 0;
  const editable = $derived(canIn(space, 'space.edit'));

  function active(prefix: string, exact = false) {
    return exact ? route.path === prefix : route.path.startsWith(prefix);
  }
  function close() { session.sidebarOpen = false; }

  async function newPage(parent: string) {
    menuFor = '';
    const title = prompt('Page title');
    if (!title?.trim()) return;
    try {
      const r = await post('/api/pages', { space, parent: parent === tree.home ? '' : parent, title });
      open = { ...open, [parent]: true };
      await loadTree();
      navigate('/pages/' + r.id + '?edit=1');
    } catch (e: any) { toast(e.message, 'error'); }
  }
  async function rename(p: any) {
    menuFor = '';
    const title = prompt('New title', p.title);
    if (!title?.trim() || title === p.title) return;
    const sibs = children(p.parent || tree.home);
    try { await post(`/api/pages/${p.id}/move`, { parent: p.parent, index: sibs.findIndex((s) => s.id === p.id), title }); loadTree(); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  async function moveTo(p: any, parent: string, index: number) {
    try { await post(`/api/pages/${p.id}/move`, { parent: parent === tree.home ? '' : parent, index }); open = { ...open, [parent]: true }; loadTree(); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  function drop(target: any, where: string) {
    const p = tree.pages.find((x) => x.id === dragId);
    dragId = ''; dropTarget = { id: '', where: '' };
    if (!p || p.id === target.id) return;
    if (where === 'inside') moveTo(p, target.id, children(target.id).length);
    else {
      const parent = target.parent || tree.home;
      const sibs = children(parent).filter((s) => s.id !== p.id);
      moveTo(p, target.parent, sibs.findIndex((s) => s.id === target.id));
    }
  }
  function over(e: DragEvent, p: any) {
    if (!dragId) return;
    e.preventDefault();
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    dropTarget = { id: p.id, where: e.clientY - r.top < r.height * 0.3 && p.id !== tree.home ? 'before' : 'inside' };
  }
  function ctx(e: MouseEvent, p: any) {
    if (!editable) return;
    e.preventDefault();
    menuFor = p.id;
  }
  const allPages = $derived(tree.pages);
  function outside(e: MouseEvent) {
    if (menuFor && !(e.target as HTMLElement).closest('.tree-menu')) menuFor = '';
  }
</script>

<svelte:window onclick={outside} />
<nav class="sidebar" class:open={session.sidebarOpen} aria-label="Main">
  <a class="nav-item" class:active={active('/', true)} href="/" onclick={close}>Home</a>
  <a class="nav-item" class:active={active('/my-work')} href="/my-work" onclick={close}>My Work</a>
  {#if can('library.view')}
    <a class="nav-item" class:active={active('/library') || active('/items')} href="/library" onclick={close}>Library</a>
    <a class="nav-item" class:active={active('/change-orders') || active('/baselines')} href="/change-orders" onclick={close}>Change orders</a>
  {/if}
  <a class="nav-item" class:active={active('/search')} href="/search" onclick={close}>Search</a>
  <a class="nav-item" class:active={active('/notifications')} href="/notifications" onclick={close}>Notifications {#if session.unread}<span class="badge-count">{session.unread}</span>{/if}</a>

  <div class="nav-section">Space</div>
  {#if spaces.length}
    <select aria-label="Space" value={space} onchange={(e) => { setSpace((e.target as HTMLSelectElement).value); navigate('/spaces/' + (e.target as HTMLSelectElement).value); close(); }} style="margin: 0 0 0.5rem">
      {#each spaces as s}<option value={s.id}>{s.name}</option>{/each}
    </select>
    <a class="nav-item" class:active={active('/spaces/' + space, true)} href="/spaces/{space}" onclick={close}>Space home</a>

    <div class="nav-section">Work</div>
    <a class="nav-item" class:active={active(`/spaces/${space}/board`)} href="/spaces/{space}/board" onclick={close}>Board</a>
    <a class="nav-item" class:active={active(`/spaces/${space}/list`)} href="/spaces/{space}/list" onclick={close}>List</a>
    <a class="nav-item" class:active={active(`/spaces/${space}/ncr-board`)} href="/spaces/{space}/ncr-board" onclick={close}>NCR Board</a>
    <a class="nav-item" class:active={active(`/spaces/${space}/ncr-list`)} href="/spaces/{space}/ncr-list" onclick={close}>NCR List</a>

    <div class="nav-section row" style="padding-right: 0.25rem">Pages <span class="spacer"></span>{#if editable}<button class="twisty" title="New page" aria-label="New page" onclick={() => newPage(tree.home)}>＋</button>{/if}</div>
    {#snippet node(p: any, depth: number)}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="tree-row" class:active={route.path === '/pages/' + p.id}
        class:drop-before={dropTarget.id === p.id && dropTarget.where === 'before'} class:drop-inside={dropTarget.id === p.id && dropTarget.where === 'inside'}
        style="padding-left: {depth * 0.9}rem" draggable={editable && p.id !== tree.home}
        ondragstart={() => (dragId = p.id)} ondragend={() => { dragId = ''; dropTarget = { id: '', where: '' }; }}
        ondragover={(e) => over(e, p)} ondrop={(e) => { e.preventDefault(); drop(p, dropTarget.where); }} oncontextmenu={(e) => ctx(e, p)}>
        {#if hasChildren(p.id)}
          <button class="twisty" aria-label={open[p.id] ? 'Collapse' : 'Expand'} onclick={() => (open = { ...open, [p.id]: !open[p.id] })}>{open[p.id] ? '▼' : '▶'}</button>
        {:else}<span style="width: 22px; display: inline-block"></span>{/if}
        <a href="/pages/{p.id}" onclick={close} title={p.title}>{p.title}</a>
        {#if editable}
          <div class="menu-wrap tree-menu">
            <button class="twisty" aria-label="Page actions" onclick={() => (menuFor = menuFor === p.id ? '' : p.id)}>⋯</button>
            {#if menuFor === p.id}
              <div class="menu">
                <button onclick={() => newPage(p.id)}>New child page</button>
                {#if p.id !== tree.home}
                  <button onclick={() => rename(p)}>Rename</button>
                  <button onclick={() => { menuFor = ''; moveFor = p; moveParent = p.parent || tree.home; }}>Move</button>
                {/if}
              </div>
            {/if}
          </div>
        {/if}
      </div>
      {#if open[p.id] || p.id === tree.home}
        {#each children(p.id) as c (c.id)}{@render node(c, depth + 1)}{/each}
      {/if}
    {/snippet}
    {#each tree.pages.filter((p) => p.id === tree.home) as h (h.id)}{@render node(h, 0)}{/each}
  {:else}
    <div class="muted small" style="padding: 0 0.65rem">You are not a member of any space.</div>
  {/if}
  {#if can('library.view')}
    <div class="nav-section">Engineering</div>
    <a class="nav-item" class:active={active('/traceability')} href="/traceability" onclick={close}>Traceability</a>
    <a class="nav-item" class:active={active('/baselines')} href="/baselines" onclick={close}>Baselines</a>
  {/if}
</nav>

{#if moveFor}
  <Modal title="Move {moveFor.title}" onclose={() => (moveFor = null)}>
    <div class="field">
      <label for="mp">New parent</label>
      <select id="mp" bind:value={moveParent}>
        {#each allPages.filter((x) => x.id !== moveFor.id) as x}<option value={x.id}>{x.title}</option>{/each}
      </select>
    </div>
    {#snippet footer()}
      <button onclick={() => (moveFor = null)}>Cancel</button>
      <button class="primary" onclick={() => { moveTo(moveFor, moveParent, children(moveParent).length); moveFor = null; }}>Move</button>
    {/snippet}
  </Modal>
{/if}
