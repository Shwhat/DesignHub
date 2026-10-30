<script lang="ts">
  import { mount, unmount, tick } from 'svelte';
  import { renderMarkdown } from '../lib/markdown';
  import LiveBlock from './LiveBlock.svelte';

  let { source }: { source: string } = $props();
  let el: HTMLElement;
  let mounted: any[] = [];
  const html = $derived(renderMarkdown(source));

  $effect(() => {
    html;
    tick().then(() => {
      for (const m of mounted) unmount(m);
      mounted = [];
      if (!el) return;
      el.querySelectorAll<HTMLElement>('[data-block]').forEach((div) => {
        div.innerHTML = '';
        mounted.push(mount(LiveBlock, { target: div, props: { kind: div.dataset.block || '', args: div.dataset.args || '' } }));
      });
    });
    return () => {
      for (const m of mounted) unmount(m);
      mounted = [];
    };
  });
</script>

<div class="markdown" bind:this={el}>{@html html}</div>
