<script lang="ts">
  import { get, post, patch } from '../lib/api';
  import { session, toast, onEvent, can } from '../lib/state.svelte';
  import { ago } from '../lib/format';
  import { onDestroy } from 'svelte';

  let { recordId, canComment = true }: { recordId: string; canComment?: boolean } = $props();
  let items: any[] = $state([]);
  let body = $state('');
  let replyTo = $state('');
  let replyBody = $state('');
  let editing = $state('');
  let editBody = $state('');
  let showResolved = $state(false);

  async function load() {
    try { items = await get(`/api/records/${encodeURIComponent(recordId)}/comments`); } catch {}
  }
  $effect(() => { recordId; load(); });
  const off = onEvent((e) => { if (e.type === 'comments' && e.id === recordId) load(); });
  onDestroy(() => off());

  async function add(text: string, reply = '') {
    if (!text.trim()) return;
    try {
      await post(`/api/records/${encodeURIComponent(recordId)}/comments`, { body: text, replyTo: reply });
      body = ''; replyBody = ''; replyTo = '';
      load();
    } catch (e: any) { toast(e.message, 'error'); }
  }
  async function update(c: any, change: any) {
    try { await patch(`/api/records/${encodeURIComponent(recordId)}/comments/${c.id}`, change); editing = ''; load(); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  const top = $derived(items.filter((c) => !c.replyTo && (showResolved || !c.resolved)));
  const replies = (id: string) => items.filter((c) => c.replyTo === id);
  const mine = (c: any) => c.authorId === session.me?.id;
</script>

<aside class="comments" aria-label="Comments">
  <div class="card-head">
    <h3>Comments</h3>
    <span class="spacer"></span>
    <label class="checkbox small"><input type="checkbox" bind:checked={showResolved} /> Resolved</label>
    <button class="ghost icon" aria-label="Close comments" onclick={() => (session.commentsOpen = false)}>✕</button>
  </div>
  <div style="flex: 1; overflow-y: auto">
    {#each top as c (c.id)}
      {#snippet one(c: any, reply: boolean)}
        <div class="comment" class:reply class:resolved={c.resolved}>
          <div class="row small"><strong>{c.author}</strong><span class="muted">{ago(c.createdAt)}{c.editedAt ? ' (edited)' : ''}</span>{#if c.revision}<span class="pill">rev {c.revision}</span>{/if}</div>
          {#if c.hidden}<div class="muted small">Comment hidden.</div>
          {:else if editing === c.id}
            <textarea bind:value={editBody}></textarea>
            <div class="row end"><button class="small" onclick={() => (editing = '')}>Cancel</button><button class="small primary" onclick={() => update(c, { body: editBody })}>Save</button></div>
          {:else}<div style="white-space: pre-wrap">{c.body}</div>{/if}
          <div class="row small" style="margin-top: 0.3rem">
            {#if !reply && canComment}<button class="ghost small" onclick={() => (replyTo = replyTo === c.id ? '' : c.id)}>Reply</button>{/if}
            {#if !reply && canComment}<button class="ghost small" onclick={() => update(c, { resolved: !c.resolved })}>{c.resolved ? 'Reopen' : 'Resolve'}</button>{/if}
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
      <div class="row end" style="margin-top: 0.4rem"><button class="primary small" disabled={!body.trim()} onclick={() => add(body)}>Comment</button></div>
    </div>
  {/if}
</aside>
