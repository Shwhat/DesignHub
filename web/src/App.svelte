<script lang="ts">
  import { session, loadSession } from './lib/state.svelte';
  import { route, match, navigate } from './lib/router.svelte';
  import Layout from './components/Layout.svelte';
  import SignIn from './pages/SignIn.svelte';
  import Home from './pages/Home.svelte';
  import MyWork from './pages/MyWork.svelte';
  import Library from './pages/Library.svelte';
  import ItemCreate from './pages/ItemCreate.svelte';
  import ItemView from './pages/ItemView.svelte';
  import ChangeOrders from './pages/ChangeOrders.svelte';
  import ChangeOrderView from './pages/ChangeOrderView.svelte';
  import Baselines from './pages/Baselines.svelte';
  import Traceability from './pages/Traceability.svelte';
  import SpaceHome from './pages/SpaceHome.svelte';
  import PageView from './pages/PageView.svelte';
  import WorkBoard from './pages/WorkBoard.svelte';
  import WorkList from './pages/WorkList.svelte';
  import WorkItemView from './pages/WorkItemView.svelte';
  import WorkCreate from './pages/WorkCreate.svelte';
  import Search from './pages/Search.svelte';
  import Notifications from './pages/Notifications.svelte';
  import Settings from './pages/Settings.svelte';
  import Admin from './pages/admin/Admin.svelte';
  import NotFound from './pages/NotFound.svelte';

  loadSession();

  type R = { pattern: string; component: any };
  const routes: R[] = [
    { pattern: '/', component: Home },
    { pattern: '/my-work', component: MyWork },
    { pattern: '/library', component: Library },
    { pattern: '/library/:tab', component: Library },
    { pattern: '/items/new', component: ItemCreate },
    { pattern: '/items/:id', component: ItemView },
    { pattern: '/change-orders', component: ChangeOrders },
    { pattern: '/change-orders/:id', component: ChangeOrderView },
    { pattern: '/baselines', component: Baselines },
    { pattern: '/traceability', component: Traceability },
    { pattern: '/spaces/:id', component: SpaceHome },
    { pattern: '/spaces/:id/board', component: WorkBoard },
    { pattern: '/spaces/:id/ncr-board', component: WorkBoard },
    { pattern: '/spaces/:id/list', component: WorkList },
    { pattern: '/spaces/:id/ncr-list', component: WorkList },
    { pattern: '/pages/:id', component: PageView },
    { pattern: '/work/new', component: WorkCreate },
    { pattern: '/work/:id', component: WorkItemView },
    { pattern: '/search', component: Search },
    { pattern: '/notifications', component: Notifications },
    { pattern: '/settings', component: Settings },
    { pattern: '/settings/:section', component: Settings },
    { pattern: '/admin', component: Admin },
    { pattern: '/admin/:section', component: Admin },
    { pattern: '/admin/:section/:id', component: Admin },
  ];

  const current = $derived.by(() => {
    for (const r of routes) {
      const params = match(r.pattern, route.path);
      if (params) return { component: r.component, params, pattern: r.pattern };
    }
    return { component: NotFound, params: {}, pattern: '' };
  });

  $effect(() => {
    if (session.loaded && !session.me && route.path !== '/signin') navigate('/signin?next=' + encodeURIComponent(route.path + route.search), true);
    if (session.loaded && session.me && route.path === '/signin') navigate(route.query.get('next') || '/', true);
    session.commentsOpen = false;
  });
  $effect(() => {
    route.path;
    session.sidebarOpen = false;
  });
</script>

{#if !session.loaded}
  <div class="empty" style="margin-top: 30vh">Loading DesignHub...</div>
{:else if route.path === '/signin' || !session.me}
  <SignIn />
{:else}
  <Layout>
    {#key route.path}
      <current.component params={current.params} pattern={current.pattern} />
    {/key}
  </Layout>
{/if}
