<script lang="ts">
  import type { Snippet } from 'svelte';
  import TopBar from './TopBar.svelte';
  import Sidebar from './Sidebar.svelte';
  import { session } from '../lib/state.svelte';
  import { date } from '../lib/format';
  const daysLeft = $derived(session.me?.expiresOn ? Math.ceil((new Date(session.me.expiresOn).getTime() - Date.now()) / 86400000) : null);
  let { children }: { children: Snippet } = $props();
</script>

<div class="shell">
  <TopBar />
  <Sidebar />
  {#if session.sidebarOpen}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="backdrop phone-backdrop" onclick={() => (session.sidebarOpen = false)}></div>
  {/if}
  <main class="main" class:with-comments={session.commentsOpen}>
    {#if daysLeft !== null && daysLeft <= 14}<div class="banner warn" style="max-width: 1280px; margin: 0 auto 12px">Your access ends on {date(session.me?.expiresOn)}.</div>{/if}
    {@render children()}
  </main>
</div>
{#if session.toast}<div class="toast" class:error={session.toast.kind === 'error'} role="status">{session.toast.text}</div>{/if}

<style>
  @media (min-width: 901px) { .phone-backdrop { display: none; } }
</style>
