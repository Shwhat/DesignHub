<script lang="ts">
  import { session, setSpace, can, canIn, onEvent, toast } from '../lib/state.svelte';
  import { route, navigate } from '../lib/router.svelte';
  import { get, post } from '../lib/api';
  import Modal from './Modal.svelte';
  import Icon from './Icon.svelte';
  import { isVendor, isContractor } from '../lib/state.svelte';
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

  let newFor: string | null = $state(null);
  let newTitle = $state('');
  let newTemplate = $state('');
  let templates: any[] = $state([]);
  async function newPage(parent: string) {
    menuFor = '';
    newFor = parent;
    newTitle = '';
    newTemplate = '';
    templates = await get(`/api/templates?space=${space}&active=1`).catch(() => []);
  }
  async function createPage() {
    const parent = newFor ?? tree.home;
    const title = newTitle;
    if (!title.trim() && !newTemplate) return;
    newFor = null;
    try {
      const r = await post('/api/pages', { space, parent: parent === tree.home ? '' : parent, title, template: newTemplate });
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
  {#if isVendor()}
  <a class="nav-item" class:active={active('/', true)} href="/" onclick={close}><Icon name="home" />Home</a>
  <a class="nav-item" class:active={active('/library') || active('/items')} href="/library" onclick={close}><Icon name="library" />Supplied items</a>
  <a class="nav-item" class:active={active('/supplier-ncrs') || active('/work')} href="/supplier-ncrs" onclick={close}><Icon name="ncr" />Nonconformances</a>
  <a class="nav-item" class:active={active('/notifications')} href="/notifications" onclick={close}><Icon name="bell" />Notifications {#if session.unread}<span class="badge-count">{session.unread}</span>{/if}</a>
  <a class="nav-item" class:active={active('/about')} href="/about" onclick={close}><Icon name="about" />About</a>
  {:else}
  <a class="nav-item" class:active={active('/', true)} href="/" onclick={close}><Icon name="home" />Home</a>
  <a class="nav-item" class:active={active('/my-work')} href="/my-work" onclick={close}><Icon name="mywork" />My Work</a>
  {#if can('library.view')}
    <a class="nav-item" class:active={active('/library') || active('/items')} href="/library" onclick={close}><Icon name="library" />Library</a>
    {#if !isContractor()}<a class="nav-item" class:active={active('/change-orders') || active('/baselines')} href="/change-orders" onclick={close}><Icon name="co" />Change orders</a>{/if}
  {/if}
  <a class="nav-item" class:active={active('/search')} href="/search" onclick={close}><Icon name="search" />Search</a>
  <a class="nav-item" class:active={active('/notifications')} href="/notifications" onclick={close}><Icon name="bell" />Notifications {#if session.unread}<span class="badge-count">{session.unread}</span>{/if}</a>
  <a class="nav-item" class:active={active('/about')} href="/about" onclick={close}><Icon name="about" />About</a>

  <div class="nav-section">Space</div>
  {#if spaces.length}
    <select aria-label="Space" value={space} onchange={(e) => { setSpace((e.target as HTMLSelectElement).value); navigate('/spaces/' + (e.target as HTMLSelectElement).value); close(); }}>
      {#each spaces as s}<option value={s.id}>{s.name}</option>{/each}
    </select>
    <a class="nav-item" class:active={active('/spaces/' + space, true)} href="/spaces/{space}" onclick={close}><Icon name="space" />Space home</a>

    <div class="nav-section">Work</div>
    <a class="nav-item" class:active={active(`/spaces/${space}/board`)} href="/spaces/{space}/board" onclick={close}><Icon name="board" />Board</a>
    <a class="nav-item" class:active={active(`/spaces/${space}/list`)} href="/spaces/{space}/list" onclick={close}><Icon name="list" />List</a>
    <a class="nav-item" class:active={active(`/spaces/${space}/ncr-board`)} href="/spaces/{space}/ncr-board" onclick={close}><Icon name="ncr" />NCR Board</a>
    <a class="nav-item" class:active={active(`/spaces/${space}/ncr-list`)} href="/spaces/{space}/ncr-list" onclick={close}><Icon name="list" />NCR List</a>
    <a class="nav-item" class:active={active(`/spaces/${space}/sprints`) || active('/sprints/')} href="/spaces/{space}/sprints" onclick={close}><Icon name="board" />Sprints</a>
    <a class="nav-item" class:active={active(`/spaces/${space}/milestones`) || active('/milestones/')} href="/spaces/{space}/milestones" onclick={close}><Icon name="baseline" />Milestones</a>
    <a class="nav-item" class:active={active(`/spaces/${space}/timeline`)} href="/spaces/{space}/timeline" onclick={close}><Icon name="trace" />Timeline</a>
    {#if canIn(space, 'time.report')}<a class="nav-item" class:active={active(`/spaces/${space}/time-report`)} href="/spaces/{space}/time-report" onclick={close}><Icon name="mywork" />Time report</a>{/if}

    <div class="nav-section row" style="padding-right: 6px">Pages <span class="spacer"></span>{#if editable}<button class="twisty" title="New page" aria-label="New page" onclick={() => newPage(tree.home)}><Icon name="plus" size={14} /></button>{/if}</div>
    {#snippet node(p: any, depth: number)}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="tree-row" class:active={route.path === '/pages/' + p.id}
        class:drop-before={dropTarget.id === p.id && dropTarget.where === 'before'} class:drop-inside={dropTarget.id === p.id && dropTarget.where === 'inside'}
        style="padding-left: {4 + depth * 14}px" draggable={editable && p.id !== tree.home}
        ondragstart={() => (dragId = p.id)} ondragend={() => { dragId = ''; dropTarget = { id: '', where: '' }; }}
        ondragover={(e) => over(e, p)} ondrop={(e) => { e.preventDefault(); drop(p, dropTarget.where); }} oncontextmenu={(e) => ctx(e, p)}>
        {#if hasChildren(p.id)}
          <button class="twisty" aria-label={open[p.id] ? 'Collapse' : 'Expand'} onclick={() => (open = { ...open, [p.id]: !open[p.id] })}><Icon name={open[p.id] ? 'down' : 'right'} size={14} /></button>
        {:else}<span style="width: 20px; flex: none"></span>{/if}
        <a href="/pages/{p.id}" onclick={close} title={p.title}><Icon name={p.id === tree.home ? 'space' : 'page'} size={14} /><span style="overflow: hidden; text-overflow: ellipsis">{p.title}</span></a>
        {#if editable}
          <div class="menu-wrap tree-menu">
            <button class="twisty" aria-label="Page actions" aria-expanded={menuFor === p.id} onclick={() => (menuFor = menuFor === p.id ? '' : p.id)}><Icon name="more" size={14} /></button>
            {#if menuFor === p.id}
              <div class="menu">
                <button onclick={() => newPage(p.id)}><Icon name="plus" />New child page...</button>
                {#if p.id !== tree.home}
                  <button onclick={() => rename(p)}>Rename...</button>
                  <button onclick={() => { menuFor = ''; moveFor = p; moveParent = p.parent || tree.home; }}>Move...</button>
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
    <div class="muted small" style="padding: 0 10px">You are not a member of any space.</div>
  {/if}
  {#if can('library.view') && !isContractor()}
    <div class="nav-section">Engineering</div>
    <a class="nav-item" class:active={active('/traceability')} href="/traceability" onclick={close}><Icon name="trace" />Traceability</a>
    <a class="nav-item" class:active={active('/baselines')} href="/baselines" onclick={close}><Icon name="baseline" />Baselines</a>
  {/if}
  {/if}
</nav>

{#if newFor !== null}
  <Modal title="New page" onclose={() => (newFor = null)}>
    <div class="field"><label for="np-t">Title</label><input id="np-t" maxlength="150" bind:value={newTitle} placeholder={newTemplate ? 'Leave empty to use the template title' : ''} /></div>
    <div class="field"><label for="np-p">Parent page</label>
      <select id="np-p" bind:value={newFor}>{#each allPages as x}<option value={x.id}>{x.id === tree.home ? x.title + ' (top level)' : x.title}</option>{/each}</select></div>
    <span class="lbl">Template</span>
    <div class="tpls">
      <label class="tpl" class:on={newTemplate === ''}><input type="radio" name="tpl" value="" bind:group={newTemplate} /><span><strong>Blank page</strong><br /><span class="muted small">Start with an empty page.</span></span></label>
      {#each templates as t}<label class="tpl" class:on={newTemplate === t.id}><input type="radio" name="tpl" value={t.id} bind:group={newTemplate} /><span><strong>{t.name}</strong>{#if t.space} <span class="pill">This space</span>{/if}<br /><span class="muted small">{t.description}</span></span></label>{/each}
    </div>
    {#snippet footer()}<button onclick={() => (newFor = null)}>Cancel</button><button class="primary" disabled={!newTitle.trim() && !newTemplate} onclick={createPage}>Create</button>{/snippet}
  </Modal>
{/if}
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

<style>
  .lbl { font-size: 11.5px; font-weight: 600; color: var(--text-2); display: block; margin-bottom: 3px; }
  .tpls { display: grid; gap: 4px; max-height: 40vh; overflow: auto; }
  .tpl { display: flex; gap: 8px; align-items: flex-start; padding: 6px 8px; border: 1px solid var(--line2); border-radius: 4px; font-weight: 400; color: var(--text); font-size: 12.5px; cursor: pointer; margin: 0; }
  .tpl.on { border-color: var(--on-line); background: var(--accent-soft); }
  .tpl input { margin-top: 3px; }
</style>
