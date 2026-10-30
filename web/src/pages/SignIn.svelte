<script lang="ts">
  import { get, post } from '../lib/api';
  import { setMe, loadUsers, connectEvents } from '../lib/state.svelte';
  import { navigate, route } from '../lib/router.svelte';

  let username = $state('');
  let password = $state('');
  let keep = $state(false);
  let error = $state('');
  let busy = $state(false);
  let message = $state('');
  let forgot = $state(false);
  get('/api/signin').then((r) => (message = r.message)).catch(() => {});

  async function submit(e: Event) {
    e.preventDefault();
    busy = true;
    error = '';
    try {
      const me = await post('/api/signin', { username, password, keep });
      setMe(me);
      loadUsers();
      connectEvents();
      navigate(route.query.get('next') || '/', true);
    } catch (err: any) {
      error = err.message;
      password = '';
    }
    busy = false;
  }
</script>

<div class="signin">
  <div class="box">
    <div class="logo"><img src="/logo.svg" alt="" width="44" height="44" /><span>DesignHub</span></div>
    {#if message}<div class="info-box" style="margin-bottom: 1rem; white-space: pre-wrap">{message}</div>{/if}
    <form onsubmit={submit} class="card card-body">
      <div class="field"><label for="u">Username</label><input id="u" autocomplete="username" bind:value={username} required /></div>
      <div class="field"><label for="p">Password</label><input id="p" type="password" autocomplete="current-password" bind:value={password} required /></div>
      <label class="checkbox" style="margin-bottom: 1rem"><input type="checkbox" bind:checked={keep} /> Keep me signed in</label>
      {#if error}<div class="error-box" style="margin-bottom: 1rem" role="alert">{error}</div>{/if}
      <button class="primary" style="width: 100%" disabled={busy}>{busy ? 'Signing in...' : 'Sign in'}</button>
    </form>
    <div style="text-align: center; margin-top: 1rem">
      <button class="ghost small" onclick={() => (forgot = !forgot)}>Forgot password</button>
      {#if forgot}<div class="muted small">Contact your administrator.</div>{/if}
    </div>
  </div>
</div>

<style>
  .signin { min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 1rem; }
  .box { width: min(400px, 100%); }
  .logo { display: flex; align-items: center; justify-content: center; gap: 0.6rem; font-size: 1.5rem; font-weight: 700; margin-bottom: 1.5rem; }
</style>
