<script lang="ts">
  import { untrack } from 'svelte';
  import { get } from '../lib/api';
  import { navigate } from '../lib/router.svelte';
  import { aboutPages } from '../lib/about';
  import Markdown from '../components/Markdown.svelte';
  import NotFound from './NotFound.svelte';

  let { params }: { params: any; pattern?: string } = $props();
  const slug = untrack(() => params.page) || aboutPages[0].slug;
  const index = aboutPages.findIndex((p) => p.slug === slug);
  const page = aboutPages[index];
  const prev = aboutPages[index - 1];
  const next = aboutPages[index + 1];

  let version = $state('');
  get('/api/signin', { background: true }).then((r) => (version = r.version)).catch(() => {});
</script>

{#if !page}
  <NotFound />
{:else}
  <div class="content">
    <div class="head">
      <h1>About DesignHub</h1>
      {#if version}<span class="muted small">Version {version}</span>{/if}
    </div>
    <div class="about">
      <nav class="card toc" aria-label="About pages">
        {#each aboutPages as p}
          <a href="/about/{p.slug}" class:active={p.slug === page.slug} aria-current={p.slug === page.slug ? 'page' : undefined}>{p.title}</a>
        {/each}
      </nav>
      <select class="toc-select" aria-label="About page" value={page.slug} onchange={(e) => navigate('/about/' + (e.target as HTMLSelectElement).value)}>
        {#each aboutPages as p}<option value={p.slug}>{p.title}</option>{/each}
      </select>
      <article class="card card-body">
        <Markdown source={page.body} />
        <div class="pager">
          {#if prev}<a href="/about/{prev.slug}">Previous: {prev.title}</a>{/if}
          <span class="spacer"></span>
          {#if next}<a href="/about/{next.slug}">Next: {next.title}</a>{/if}
        </div>
      </article>
    </div>
  </div>
{/if}

<style>
  .head { display: flex; align-items: baseline; gap: 0.75rem; flex-wrap: wrap; }
  .about { display: grid; grid-template-columns: 220px minmax(0, 1fr); gap: 1rem; align-items: start; }
  .toc { display: flex; flex-direction: column; padding: 0.4rem; position: sticky; top: 1rem; }
  .toc a { padding: 0.45rem 0.65rem; border-radius: 6px; color: var(--text-2); text-decoration: none; }
  .toc a:hover { background: var(--surface-2); color: var(--text); }
  .toc a.active { background: var(--surface-2); color: var(--accent); font-weight: 600; }
  .toc-select { display: none; }
  article :global(.markdown > h1:first-child) { margin-top: 0; }
  .pager { display: flex; gap: 1rem; flex-wrap: wrap; border-top: 1px solid var(--border); margin-top: 1.5rem; padding-top: 0.75rem; }
  .spacer { flex: 1; }
  @media (max-width: 900px) {
    .about { grid-template-columns: minmax(0, 1fr); }
    .toc { display: none; }
    .toc-select { display: block; width: 100%; }
  }
</style>
