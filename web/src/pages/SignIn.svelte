<script lang="ts">
  import { get, post } from '../lib/api';
  import { setMe, loadUsers, connectEvents } from '../lib/state.svelte';
  import { navigate, route } from '../lib/router.svelte';

  const googleErrors: Record<string, string> = {
    google_not_allowed: 'That Google account is not allowed to use DesignHub. Sign in with your company account, or ask your administrator to add your address.',
    google_inactive: 'Your DesignHub account is disabled. Contact your administrator.',
    google_duplicate: 'More than one DesignHub account uses this email address. Contact your administrator.',
    google_cancelled: 'Google sign-in was cancelled.',
    google_expired: 'The sign-in request expired. Try again.',
    google_off: 'Google sign-in is not turned on.',
    google_busy: 'Too many sign-in requests in progress. Try again in a few minutes.',
    google_failed: 'Google sign-in failed. Try again, or contact your administrator if it keeps happening.',
  };

  let username = $state('');
  let password = $state('');
  let keep = $state(false);
  let error = $state(googleErrors[route.query.get('error') || ''] || '');
  let busy = $state(false);
  let message = $state('');
  let google = $state(false);
  let passwordOff = $state(false);
  let showPassword = $state(false);
  let forgot = $state(false);
  let loaded = $state(false);
  get('/api/signin')
    .then((r) => { message = r.message; google = !!r.google; passwordOff = !!r.passwordOff; })
    .catch(() => {})
    .finally(() => (loaded = true));

  function googleHref() {
    const next = route.query.get('next') || '/';
    return `/api/auth/google/start?next=${encodeURIComponent(next)}${keep ? '&keep=1' : ''}`;
  }

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
    {#if loaded}
      <div class="card card-body">
        {#if google}
          <a class="gbtn" href={googleHref()}>
            <svg viewBox="0 0 48 48" width="18" height="18" aria-hidden="true">
              <path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z" />
              <path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z" />
              <path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z" />
              <path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z" />
            </svg>
            <span>Sign in with Google</span>
          </a>
          <label class="checkbox" style="margin: 1rem 0 0"><input type="checkbox" bind:checked={keep} /> Keep me signed in</label>
        {/if}
        {#if error}<div class="error-box" style="margin-top: 1rem" role="alert">{error}</div>{/if}
        {#if google && !showPassword}
          <div class="alt"><button class="ghost small" onclick={() => (showPassword = true)}>{passwordOff ? 'Recovery sign-in' : 'Sign in with a password'}</button></div>
        {/if}
        {#if !google || showPassword}
          <form onsubmit={submit} style={google ? 'margin-top: 1rem' : ''}>
            {#if passwordOff}<p class="small muted">Password sign-in is turned off. Only a Super Admin given a recovery sign-in from the server can use it.</p>{/if}
            <div class="field"><label for="u">Username</label><input id="u" autocomplete="username" bind:value={username} required /></div>
            <div class="field"><label for="p">Password</label><input id="p" type="password" autocomplete="current-password" bind:value={password} required /></div>
            {#if !google}<label class="checkbox" style="margin-bottom: 1rem"><input type="checkbox" bind:checked={keep} /> Keep me signed in</label>{/if}
            <button class="primary" style="width: 100%" disabled={busy}>{busy ? 'Signing in...' : 'Sign in'}</button>
          </form>
        {/if}
      </div>
      {#if !passwordOff}
        <div style="text-align: center; margin-top: 1rem">
          <button class="ghost small" onclick={() => (forgot = !forgot)}>Forgot password</button>
          {#if forgot}<div class="muted small">Contact your administrator.</div>{/if}
        </div>
      {/if}
    {/if}
  </div>
</div>

<style>
  .signin { min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 1rem; }
  .box { width: min(400px, 100%); }
  .logo { display: flex; align-items: center; justify-content: center; gap: 0.6rem; font-size: 1.5rem; font-weight: 700; margin-bottom: 1.5rem; }
  .gbtn {
    display: flex; align-items: center; justify-content: center; gap: 10px; width: 100%; height: 40px;
    background: #fff; color: #1f1f1f; border: 1px solid #747775; border-radius: 4px;
    font-family: Roboto, Arial, sans-serif; font-size: 14px; font-weight: 500; text-decoration: none;
  }
  .gbtn:hover { background: #f8f9fa; text-decoration: none; }
  .gbtn:focus-visible { outline: 2px solid #4285f4; outline-offset: 2px; }
  .alt { text-align: center; margin-top: 0.75rem; }
</style>
