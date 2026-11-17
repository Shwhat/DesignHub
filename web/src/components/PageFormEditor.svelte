<script lang="ts">
  import FormFill from './FormFill.svelte';
  import Markdown from './Markdown.svelte';
  import { render, missing, type FormField } from '../lib/forms';

  // A page form with the generated markdown shown live below it. body and
  // miss are outputs.
  let { layout, fields, values = $bindable(), title, body = $bindable(''), miss = $bindable([]), showErrors = false }:
    { layout: string; fields: FormField[]; values: Record<string, any>; title: string; body?: string; miss?: any[]; showErrors?: boolean } = $props();

  let view: 'preview' | 'markdown' = $state('preview');
  $effect(() => { body = render(layout, fields, values, title); });
  $effect(() => { miss = missing(fields, values); });
</script>

<div class="pfe">
  <section class="card card-body">
    <FormFill {fields} bind:values miss={miss} {showErrors} />
  </section>
  {#if showErrors && miss.length}
    <div class="banner warn">Fill in the required fields: {miss.map((m) => m.label).join(', ')}.</div>
  {/if}
  <section class="card">
    <div class="card-head row">
      <h3 style="margin: 0">Document</h3><span class="spacer"></span>
      <div class="tabs" style="margin: 0; border: 0">
        <button class:active={view === 'preview'} onclick={() => (view = 'preview')}>Preview</button>
        <button class:active={view === 'markdown'} onclick={() => (view = 'markdown')}>Markdown</button>
      </div>
    </div>
    <div class="card-body">
      {#if view === 'preview'}<Markdown source={body} />{:else}<pre class="md-source">{body}</pre>{/if}
    </div>
  </section>
</div>

<style>
  .pfe { display: grid; gap: 12px; }
  .md-source { white-space: pre-wrap; margin: 0; font-size: 12px; font-family: ui-monospace, Menlo, Consolas, monospace; background: var(--panel); border: 1px solid var(--line2); border-radius: 4px; padding: 8px 10px; }
</style>
