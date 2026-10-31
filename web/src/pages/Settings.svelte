<script lang="ts">
  import { untrack } from 'svelte';
  import { get, patch, post } from '../lib/api';
  import { session, setMe, toast } from '../lib/state.svelte';
  import { navigate } from '../lib/router.svelte';
  import { dateTime, ago } from '../lib/format';
  import { HOME_SECTIONS, homeLayout } from '../lib/home';
  let { params }: { params: any; pattern?: string } = $props();
  const section = untrack(() => params.section) || 'profile';
  const me = session.me!;
  let p = $state({ displayName: me.displayName, email: me.email, timeZone: me.timeZone, editMode: me.editMode, theme: me.theme });
  let layout = $state(homeLayout(me.homeLayout));
  let notify: Record<string, boolean> = $state({ ...(me.notifyEvents || {}) });
  let pw = $state({ current: '', new: '', repeat: '' });
  let sessions: any[] = $state([]);
  const events = ['Assigned', 'Mentioned', 'Status changed', 'Approval requested', 'Approval decided', 'Due soon', 'Overdue', 'Watched record changed'];
  const zones = (Intl as any).supportedValuesOf ? (Intl as any).supportedValuesOf('timeZone') : ['UTC', 'America/Los_Angeles', 'America/New_York', 'Europe/London'];
  async function save(body: any) {
    try { setMe({ ...(await patch('/api/me', body)), csrf: me.csrf } as any); toast('Saved.'); } catch (e: any) { toast(e.message, 'error'); }
  }
  async function changePassword() {
    if (pw.new !== pw.repeat) { toast('The new passwords do not match.', 'error'); return; }
    try { await post('/api/me/password', { current: pw.current, new: pw.new }); pw = { current: '', new: '', repeat: '' }; toast('Password changed. Other devices were signed out.'); }
    catch (e: any) { toast(e.message, 'error'); }
  }
  async function loadSessions() { sessions = await get('/api/me/sessions'); }
  if (section === 'security') loadSessions();
  function move(i: number, d: number) { const l = [...layout]; const j = i + d; if (j < 0 || j >= l.length) return; [l[i], l[j]] = [l[j], l[i]]; layout = l; }
  const sections = [['profile', 'Profile'], ['security', 'Security'], ['appearance', 'Appearance'], ['editing', 'Editing'], ['home', 'Home layout'], ['notifications', 'Notifications']];
</script>

<div class="content narrow">
  <h1>Settings</h1>
  <div class="tabs">{#each sections as [k, l]}<button class:active={section === k} onclick={() => navigate('/settings/' + k)}>{l}</button>{/each}</div>
  <div class="card card-body">
    {#if section === 'profile'}
      <div class="field"><label for="dn">Display name</label><input id="dn" bind:value={p.displayName} /></div>
      <div class="field"><label for="em">Email</label><input id="em" type="email" bind:value={p.email} /></div>
      <div class="field"><label for="tz">Time zone</label><select id="tz" bind:value={p.timeZone}><option value="">Follow this device</option>{#each zones as z}<option>{z}</option>{/each}</select></div>
      <div class="row end"><button class="primary" onclick={() => save({ displayName: p.displayName, email: p.email, timeZone: p.timeZone })}>Save</button></div>
    {:else if section === 'security'}
      <h3>Change password</h3>
      <div class="field"><label for="pc">Current password</label><input id="pc" type="password" autocomplete="current-password" bind:value={pw.current} /></div>
      <div class="field"><label for="pn">New password (at least 10 characters)</label><input id="pn" type="password" autocomplete="new-password" bind:value={pw.new} /></div>
      <div class="field"><label for="pr">Repeat new password</label><input id="pr" type="password" autocomplete="new-password" bind:value={pw.repeat} /></div>
      <div class="row end"><button class="primary" disabled={!pw.current || !pw.new} onclick={changePassword}>Change password</button></div>
      <hr />
      <div class="row"><h3 style="margin: 0">Active sessions</h3><span class="spacer"></span><button onclick={async () => { await post('/api/me/sessions/signout-others'); toast('Other devices signed out.'); loadSessions(); }}>Sign out other devices</button></div>
      <table class="data" style="margin-top: 0.75rem"><tbody>
        {#each sessions as s}<tr><td>{s.userAgent.slice(0, 60)}{#if s.current} <span class="pill">this device</span>{/if}</td><td>{s.ip}</td><td>{ago(s.lastSeen)}</td></tr>{/each}
      </tbody></table>
    {:else if section === 'appearance'}
      <div class="field"><span class="small muted">Theme</span>
        {#each [['light', 'Light'], ['dark', 'Dark'], ['device', 'Follow device']] as [k, l]}<label class="checkbox"><input type="radio" name="theme" value={k} bind:group={p.theme} onchange={() => save({ theme: p.theme })} /> {l}</label>{/each}</div>
    {:else if section === 'editing'}
      <div class="field"><label for="ed">Default editing mode</label><select id="ed" bind:value={p.editMode} onchange={() => save({ editMode: p.editMode })}><option value="visual">Visual</option><option value="split">Split</option><option value="source">Source</option><option value="preview">Preview toggle</option></select></div>
    {:else if section === 'home'}
      {#each layout as s, i}<div class="row" style="margin-bottom: 0.4rem"><label class="checkbox" style="flex: 1"><input type="checkbox" bind:checked={s.on} /> {HOME_SECTIONS.find((h) => h.key === s.key)?.title}</label><button class="small" onclick={() => move(i, -1)}>↑</button><button class="small" onclick={() => move(i, 1)}>↓</button></div>{/each}
      <div class="row end"><button class="primary" onclick={() => save({ homeLayout: layout })}>Save</button></div>
    {:else if section === 'notifications'}
      {#each events as e}<label class="checkbox" style="margin-bottom: 0.4rem"><input type="checkbox" checked={notify[e] !== false} onchange={(x) => (notify[e] = (x.target as HTMLInputElement).checked)} /> {e}</label>{/each}
      <div class="row end"><button class="primary" onclick={() => save({ notifyEvents: notify })}>Save</button></div>
    {/if}
  </div>
</div>
