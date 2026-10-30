<script lang="ts">
  import Modal from './Modal.svelte';
  let { title, decision, requireComment = false, onsign, onclose }:
    { title: string; decision: string; requireComment?: boolean; onsign: (password: string, comment: string) => Promise<void>; onclose: () => void } = $props();
  let password = $state('');
  let comment = $state('');
  let error = $state('');
  let busy = $state(false);
  async function sign() {
    busy = true; error = '';
    try { await onsign(password, comment); }
    catch (e: any) { error = e.message; password = ''; }
    busy = false;
  }
</script>

<Modal {title} {onclose}>
  <p class="small muted">Your password confirms this decision as your electronic signature. The decision, time and comment are recorded.</p>
  <div class="field"><label for="sigc">Comment{requireComment ? '' : ' (optional)'}</label><textarea id="sigc" bind:value={comment}></textarea></div>
  <div class="field"><label for="sigp">Password</label><input id="sigp" type="password" autocomplete="current-password" bind:value={password} onkeydown={(e) => e.key === 'Enter' && password && sign()} /></div>
  {#if error}<div class="error-box">{error}</div>{/if}
  {#snippet footer()}
    <button onclick={onclose}>Cancel</button>
    <button class={decision === 'Rejected' ? 'danger' : 'primary'} disabled={busy || !password || (requireComment && !comment.trim())} onclick={sign}>{decision === 'Rejected' ? 'Reject' : decision === 'Release' ? 'Release' : 'Approve'}</button>
  {/snippet}
</Modal>
