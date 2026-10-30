<script lang="ts">
  import type { Snippet } from 'svelte';
  import TopBar from './TopBar.svelte';
  import Sidebar from './Sidebar.svelte';
  import { session } from '../lib/state.svelte';
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
    {@render children()}
  </main>
</div>
{#if session.toast}<div class="toast" class:error={session.toast.kind === 'error'} role="status">{session.toast.text}</div>{/if}

<style>
  @media (min-width: 901px) { .phone-backdrop { display: none; } }
</style>
