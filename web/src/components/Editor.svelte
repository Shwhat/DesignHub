<script lang="ts">
  // Markdown editor with four modes over the same text: Visual (Milkdown),
  // Split, Source and Preview toggle (CodeMirror).
  import { onDestroy, tick } from 'svelte';
  import { EditorView, keymap, placeholder as cmPlaceholder } from '@codemirror/view';
  import { EditorState } from '@codemirror/state';
  import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
  import { markdown } from '@codemirror/lang-markdown';
  import { syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language';
  import { upload, fileURL, put } from '../lib/api';
  import { session, toast } from '../lib/state.svelte';
  import Markdown from './Markdown.svelte';
  import BlockDialog from './BlockDialog.svelte';

  type Mode = 'visual' | 'split' | 'source' | 'preview';
  let { value = $bindable(''), recordId, mode = $bindable(session.me?.editMode || 'visual') }: { value: string; recordId: string; mode?: Mode } = $props();

  let cmHost: HTMLElement | undefined = $state();
  let visualHost: HTMLElement | undefined = $state();
  let view: EditorView | null = null;
  let crepe: any = null;
  let showPreview = $state(false);
  let blockDialog = $state(false);
  let fileInput: HTMLInputElement;
  let busy = $state(false);

  // Uploads a file and returns the markdown that references it.
  async function attach(file: File): Promise<string> {
    busy = true;
    try {
      const f = await upload(file);
      const url = fileURL(recordId, f, true);
      return f.mime.startsWith('image/') && f.mime !== 'image/svg+xml' ? `![${f.name}](${url})` : `[${f.name}](${fileURL(recordId, f)})`;
    } catch (e: any) {
      toast(e.message, 'error');
      return '';
    } finally {
      busy = false;
    }
  }

  function cmInsert(text: string) {
    if (!view) return;
    const r = view.state.selection.main;
    view.dispatch({ changes: { from: r.from, to: r.to, insert: text }, selection: { anchor: r.from + text.length } });
    view.focus();
  }

  function wrap(before: string, after = before) {
    if (!view) return;
    const r = view.state.selection.main;
    const sel = view.state.sliceDoc(r.from, r.to);
    view.dispatch({ changes: { from: r.from, to: r.to, insert: before + sel + after }, selection: { anchor: r.from + before.length, head: r.from + before.length + sel.length } });
    view.focus();
  }

  function linePrefix(prefix: string) {
    if (!view) return;
    const line = view.state.doc.lineAt(view.state.selection.main.from);
    view.dispatch({ changes: { from: line.from, insert: prefix } });
    view.focus();
  }

  function mountCM() {
    if (!cmHost || view) return;
    view = new EditorView({
      parent: cmHost,
      state: EditorState.create({
        doc: value,
        extensions: [
          history(),
          keymap.of([
            ...defaultKeymap,
            ...historyKeymap,
            { key: 'Mod-b', run: () => (wrap('**'), true) },
            { key: 'Mod-i', run: () => (wrap('_'), true) },
          ]),
          markdown(),
          syntaxHighlighting(defaultHighlightStyle),
          EditorView.lineWrapping,
          cmPlaceholder('Write in markdown. Type / at the start of a line to insert a live block.'),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) value = u.state.doc.toString();
          }),
          EditorView.domEventHandlers({
            keydown: (e, v) => {
              if (e.key === '/') {
                const line = v.state.doc.lineAt(v.state.selection.main.from);
                if (line.text.trim() === '') {
                  e.preventDefault();
                  blockDialog = true;
                  return true;
                }
              }
              return false;
            },
            paste: (e) => {
              const files = Array.from(e.clipboardData?.files || []);
              if (!files.length) return false;
              e.preventDefault();
              files.forEach(async (f) => cmInsert(await attach(f)));
              return true;
            },
            drop: (e, v) => {
              const files = Array.from(e.dataTransfer?.files || []);
              if (!files.length) return false;
              e.preventDefault();
              const pos = v.posAtCoords({ x: e.clientX, y: e.clientY });
              if (pos !== null) v.dispatch({ selection: { anchor: pos } });
              files.forEach(async (f) => cmInsert(await attach(f)));
              return true;
            },
          }),
        ],
      }),
    });
  }

  async function mountVisual() {
    if (!visualHost || crepe) return;
    const { Crepe } = await import('@milkdown/crepe');
    await import('@milkdown/crepe/theme/common/style.css');
    await import('@milkdown/crepe/theme/frame.css');
    const host = visualHost;
    if (!host || mode !== 'visual') return;
    const c = new Crepe({
      root: host,
      defaultValue: value,
      features: { [Crepe.Feature.Latex]: false, [Crepe.Feature.AI]: false, [Crepe.Feature.TopBar]: false },
      featureConfigs: {
        [Crepe.Feature.ImageBlock]: {
          onUpload: async (file: File) => {
            const f = await upload(file);
            return fileURL(recordId, f, true);
          },
        },
        [Crepe.Feature.Placeholder]: { text: 'Write here. Type / for headings, lists, tables and more.' },
      } as any,
    });
    c.on((l: any) =>
      l.markdownUpdated((_: any, md: string) => {
        value = md;
      }),
    );
    await c.create();
    if (mode !== 'visual') {
      c.destroy();
      return;
    }
    crepe = c;
  }

  function unmountAll() {
    view?.destroy();
    view = null;
    if (crepe) {
      crepe.destroy();
      crepe = null;
    }
  }

  // Switching modes hands the same markdown to the other editor.
  $effect(() => {
    const m = mode;
    unmountAll();
    showPreview = false;
    tick().then(() => {
      if (m === 'visual') mountVisual();
      else mountCM();
    });
  });

  // External value changes (restore, conflict reload) refresh the editor.
  export function setValue(v: string) {
    value = v;
    if (view && view.state.doc.toString() !== v) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: v } });
    if (crepe) {
      unmountAll();
      tick().then(mountVisual);
    }
  }

  async function insertBlock(md: string) {
    blockDialog = false;
    if (mode === 'visual' && crepe) {
      const { insert } = await import('@milkdown/kit/utils');
      crepe.editor.action(insert(md));
    } else cmInsert(md);
  }

  async function pickFile(e: Event) {
    const files = Array.from((e.target as HTMLInputElement).files || []);
    for (const f of files) {
      const md = await attach(f);
      if (!md) continue;
      if (mode === 'visual' && crepe) {
        const { insert } = await import('@milkdown/kit/utils');
        crepe.editor.action(insert(md, true));
      } else cmInsert(md);
    }
    (e.target as HTMLInputElement).value = '';
  }

  function saveMode() {
    put('/api/prefs/editor.mode', mode).catch(() => {});
  }

  onDestroy(unmountAll);
</script>

<div class="editor-wrap">
  <div class="editor-toolbar">
    <select bind:value={mode} onchange={saveMode} aria-label="Editing mode">
      <option value="visual">Visual</option>
      <option value="split">Split</option>
      <option value="source">Source</option>
      <option value="preview">Preview toggle</option>
    </select>
    {#if mode !== 'visual'}
      <button title="Bold" onclick={() => wrap('**')}><b>B</b></button>
      <button title="Italic" onclick={() => wrap('_')}><i>I</i></button>
      <button title="Heading" onclick={() => linePrefix('## ')}>H</button>
      <button title="Bulleted list" onclick={() => linePrefix('- ')}>•</button>
      <button title="Numbered list" onclick={() => linePrefix('1. ')}>1.</button>
      <button title="Link" onclick={() => wrap('[', '](url)')}>Link</button>
      <button title="Code" onclick={() => wrap('`')}>{'</>'}</button>
      <button title="Table" onclick={() => cmInsert('\n| Column | Column |\n| --- | --- |\n| | |\n')}>Table</button>
    {/if}
    <button onclick={() => fileInput.click()} disabled={busy}>{busy ? 'Uploading...' : 'Attach'}</button>
    <input type="file" multiple hidden bind:this={fileInput} onchange={pickFile} />
    <button onclick={() => (blockDialog = true)}>Insert block</button>
    {#if mode === 'preview'}
      <span class="spacer"></span>
      <button class:primary={showPreview} onclick={() => (showPreview = !showPreview)}>{showPreview ? 'Edit' : 'Preview'}</button>
    {/if}
  </div>
  {#if mode === 'visual'}
    <div class="visual-host" bind:this={visualHost}></div>
  {:else if mode === 'split'}
    <div class="editor-split">
      <div class="cm-host" bind:this={cmHost}></div>
      <div class="preview-pane"><Markdown source={value} /></div>
    </div>
  {:else}
    <div class="cm-host" bind:this={cmHost} hidden={mode === 'preview' && showPreview}></div>
    {#if mode === 'preview' && showPreview}<div style="padding: 0.75rem 1rem; min-height: 360px"><Markdown source={value} /></div>{/if}
  {/if}
</div>

{#if blockDialog}<BlockDialog oninsert={insertBlock} onclose={() => (blockDialog = false)} />{/if}

<style>
  :global(.milkdown) {
    --crepe-color-background: var(--surface);
    --crepe-color-on-background: var(--text);
    --crepe-color-surface: var(--surface-2);
    --crepe-color-surface-low: var(--surface-3);
    --crepe-color-on-surface: var(--text);
    --crepe-color-on-surface-variant: var(--text-2);
    --crepe-color-outline: var(--border-strong);
    --crepe-color-primary: var(--accent);
    --crepe-color-secondary: var(--accent-soft);
    --crepe-color-on-secondary: var(--text);
    --crepe-color-hover: var(--surface-2);
    --crepe-color-selected: var(--accent-soft);
    --crepe-color-inline-area: var(--surface-3);
    --crepe-font-title: var(--font);
    --crepe-font-default: var(--font);
    --crepe-font-code: var(--mono);
    --crepe-base-font-size: 15px;
  }
  :global(.milkdown .ProseMirror) {
    padding: 1rem 1.25rem 2rem 3.5rem !important;
    min-height: 340px;
  }
</style>
