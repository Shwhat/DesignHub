<script lang="ts">
  import Icon from './Icon.svelte';
  import { get, post, patch } from '../lib/api';
  import { session, toast, onEvent, can } from '../lib/state.svelte';
  import { ago } from '../lib/format';
  import { onDestroy, tick } from 'svelte';
  import { inline } from '../lib/inline.svelte';
  import { isVendor } from '../lib/state.svelte';

  let { recordId, canComment = true }: { recordId: string; canComment?: boolean } = $props();
  let items: any[] = $state([]);
  let body = $state('');
  let replyTo = $state('');
  let replyBody = $state('');
  let editing = $state('');
  let editBody = $state('');
  let showResolved = $state(false);
  let internal = $state(false);
  let anchorBody = $state('');
  let listEl: HTMLElement | undefined = $state();
  $effect(() => {
    const id = inline.focus;
    if (!id || !listEl) return;
    tick().then(() => listEl?.querySelector(`[data-cid="${id}"]`)?.scrollIntoView({ block: 'center' }));
  });

  async function load() {
    try { items = await get(`/api/records/${encodeURIComponent(recordId)}/comments`); } catch {}
  }
  $effect(() => { recordId; load(); });
  const off = onEvent((e) => { if (e.type === 'comments' && e.id === recordId) load(); });
  onDestroy(() => off());

  async function add(text: string, reply = '') {
    if (!text.trim()) return;
    try {
      await post(`/api/records/${encodeURIComponent(recordId)}/comments`, { body: text, replyTo: reply, internal: !reply && internal });
      body = ''; replyBody = ''; replyTo = ''; internal = false;
      await load();
      inline.version++;
    } catch (e: any) { toast(e.message, 'error'); }
  }
  async function update(c: any, change: any) {
    try { await patch(`/api/records/${encodeURIComponent(recordId)}/comments/${c.id}`, change); editing = ''; load(); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  async function addInline() {
    if (!anchorBody.trim() || !inline.pending) return;
    try {
      await post(`/api/records/${encodeURIComponent(recordId)}/comments`, { body: anchorBody, anchor: inline.pending, internal });
      anchorBody = ''; inline.pending = null; internal = false;
      await load();
      inline.version++;
    } catch (e: any) { toast(e.message, 'error'); }
  }
  // Inline comments first, in document order, then page comments.
  const norm = (t: string) => t.replace(/\s+/g, ' ').trim();
  const pos = (c: any) => (c.anchor && inline.text ? norm(inline.text).indexOf(norm(c.anchor.quote)) : -1);
  const top = $derived(items.filter((c) => !c.replyTo && (showResolved || !c.resolved)).sort((a, b) => {
    if (!!a.anchor !== !!b.anchor) return a.anchor ? -1 : 1;
    if (a.anchor && b.anchor) return pos(a) - pos(b);
    return 0;
  }));
  const changed = (c: any) => !!c.anchor && !!inline.text && pos(c) < 0;
  const replies = (id: string) => items.filter((c) => c.replyTo === id);
  const mine = (c: any) => c.authorId === session.me?.id;
</script>

<aside class="comments" aria-label="Comments">
  <div class="card-head">
    <h3>Comments</h3>
    <span class="spacer"></span>
    <label class="checkbox small"><input type="checkbox" bind:checked={showResolved} /> Resolved</label>
    <button class="ghost icon" aria-label="Close comments" onclick={() => (session.commentsOpen = false)}><Icon name="close" /></button>
  </div>
  {#if inline.pending && canComment}
    <div class="comment" style="background: var(--warn-soft)">
      <div class="quote">{inline.pending.quote}</div>
      <textarea bind:value={anchorBody} placeholder="Comment on the selected text"></textarea>
      <div class="row end" style="margin-top: 4px"><button class="small" onclick={() => (inline.pending = null)}>Cancel</button><button class="small primary" disabled={!anchorBody.trim()} onclick={addInline}>Comment</button></div>
    </div>
  {/if}
  <div style="flex: 1; overflow-y: auto" bind:this={listEl}>
    {#each top as c (c.id)}
      {#snippet one(c: any, reply: boolean)}
        <div class="comment" class:reply class:resolved={c.resolved} class:focus={inline.focus === c.id} data-cid={c.id}>
          {#if c.anchor}<button class="quote" title="Show in the text" onclick={() => (inline.focus = c.id)}>{c.anchor.quote}</button>{#if changed(c)}<div class="small" style="color: var(--warn)">Text changed</div>{/if}{/if}
          <div class="row small"><strong>{c.author}</strong>{#if c.internal}<span class="pill" title="Hidden from Vendors">Internal</span>{/if}<span class="muted">{ago(c.createdAt)}{c.editedAt ? ' (edited)' : ''}</span>{#if c.revision}<span class="pill">rev {c.revision}</span>{/if}</div>
          {#if c.hidden}<div class="muted small">Comment hidden.</div>
          {:else if editing === c.id}
            <textarea bind:value={editBody}></textarea>
            <div class="row end"><button class="small" onclick={() => (editing = '')}>Cancel</button><button class="small primary" onclick={() => update(c, { body: editBody })}>Save</button></div>
          {:else}<div style="white-space: pre-wrap">{c.body}</div>{/if}
          <div class="row small" style="margin-top: 0.3rem">
            {#if !reply && canComment}<button class="ghost small" onclick={() => (replyTo = replyTo === c.id ? '' : c.id)}>Reply</button>{/if}
            {#if !reply && canComment}<button class="ghost small" onclick={async () => { await update(c, { resolved: !c.resolved }); inline.version++; }}>{c.resolved ? 'Reopen' : 'Resolve'}</button>{/if}
            {#if !reply && !isVendor() && mine(c)}<button class="ghost small" onclick={() => update(c, { internal: !c.internal })}>{c.internal ? 'Share with Vendors' : 'Mark internal'}</button>{/if}
            {#if mine(c) && !c.hidden}<button class="ghost small" onclick={() => { editing = c.id; editBody = c.body; }}>Edit</button>{/if}
            {#if (mine(c) || can('users.manage'))}<button class="ghost small" onclick={() => update(c, { hidden: !c.hidden })}>{c.hidden ? 'Show' : 'Hide'}</button>{/if}
          </div>
        </div>
      {/snippet}
      {@render one(c, false)}
      {#each replies(c.id) as r (r.id)}{@render one(r, true)}{/each}
      {#if replyTo === c.id}
        <div class="comment reply">
          <textarea bind:value={replyBody} placeholder="Reply. Use @username to mention someone."></textarea>
          <div class="row end"><button class="small primary" onclick={() => add(replyBody, c.id)}>Reply</button></div>
        </div>
      {/if}
    {:else}
      <div class="empty">No comments yet.</div>
    {/each}
  </div>
  {#if canComment}
    <div style="padding: 0.75rem; border-top: 1px solid var(--border)">
      <textarea bind:value={body} placeholder="Add a comment. Use @username to mention someone."></textarea>
      <div class="row end" style="margin-top: 0.4rem">{#if !isVendor()}<label class="checkbox small" style="margin-right: auto" title="Internal comments are hidden from Vendors"><input type="checkbox" bind:checked={internal} /> Internal</label>{/if}<button class="primary small" disabled={!body.trim()} onclick={() => add(body)}>Comment</button></div>
    </div>
  {/if}
</aside>

<style>
  .quote { display: block; width: 100%; text-align: left; border: 0; border-left: 3px solid #e0a100; background: var(--warn-soft); color: var(--text); padding: 3px 7px; margin-bottom: 4px; font-style: italic; white-space: normal; border-radius: 0; min-height: 0; max-height: 4.2em; overflow: hidden; }
  .comment.focus { box-shadow: inset 3px 0 0 var(--accent); }
</style>
