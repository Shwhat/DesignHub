// Application state shared by all screens.
import { get, setCsrf, setSignedOutHandler } from './api';
import { navigate, route } from './router.svelte';

export interface SpaceRef {
  id: string;
  name: string;
  role: string;
  permissions: string[];
  archived: boolean;
  homePage: string;
}

export interface Me {
  id: string;
  username: string;
  displayName: string;
  email: string;
  libraryRole: string;
  permissions: string[];
  editMode: 'visual' | 'split' | 'source' | 'preview';
  timeZone: string;
  theme: 'light' | 'dark' | 'device';
  homeLayout: { key: string; on: boolean }[];
  notifyEvents: Record<string, boolean>;
  spaces: SpaceRef[];
  csrf: string;
  canManageApprovers: boolean;
  signInMethod: string;
}

export interface UserRef {
  id: string;
  username: string;
  displayName: string;
  libraryRole: string;
  active: boolean;
  email?: string;
}

export const session = $state({
  me: null as Me | null,
  loaded: false,
  unread: 0,
  space: '' as string, // current space for the sidebar
  sidebarOpen: false,
  commentsOpen: false,
  users: [] as UserRef[],
  toast: null as { text: string; kind: 'ok' | 'error' } | null,
});

export function can(p: string) {
  return !!session.me?.permissions.includes(p);
}

export function canIn(space: string, p: string) {
  return !!session.me?.spaces.find((s) => s.id === space)?.permissions?.includes(p);
}

export function userName(idOrUsername: string | null | undefined) {
  if (!idOrUsername) return '';
  const u = session.users.find((x) => x.id === idOrUsername || x.username.toLowerCase() === idOrUsername.toLowerCase());
  return u ? u.displayName : idOrUsername;
}

export function spaceName(id: string) {
  return session.me?.spaces.find((s) => s.id === id)?.name || id;
}

let toastTimer: any;
export function toast(text: string, kind: 'ok' | 'error' = 'ok') {
  session.toast = { text, kind };
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (session.toast = null), kind === 'error' ? 7000 : 3500);
}

export function setMe(me: Me | null) {
  session.me = me;
  if (me) {
    setCsrf(me.csrf);
    if (!session.space || !me.spaces.find((s) => s.id === session.space)) {
      let saved = '';
      try {
        saved = localStorage.getItem('dh.space') || '';
      } catch {}
      const pick = me.spaces.find((s) => s.id === saved && !s.archived) || me.spaces.find((s) => !s.archived);
      session.space = pick?.id || '';
    }
  }
  applyTheme();
}

export function setSpace(id: string) {
  session.space = id;
  try {
    localStorage.setItem('dh.space', id);
  } catch {}
}

const media = typeof window !== 'undefined' ? window.matchMedia('(prefers-color-scheme: dark)') : null;
media?.addEventListener('change', () => applyTheme());

export function applyTheme() {
  const pref = session.me?.theme || 'device';
  const dark = pref === 'dark' || (pref === 'device' && media?.matches);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
}

export async function loadSession() {
  try {
    setMe(await get<Me>('/api/me'));
    loadUsers();
    connectEvents();
  } catch {
    setMe(null);
  }
  session.loaded = true;
}

export async function loadUsers() {
  try {
    session.users = await get<UserRef[]>('/api/users', { background: true });
  } catch {}
}

export async function refreshMe() {
  try {
    setMe(await get<Me>('/api/me', { background: true }));
  } catch {}
}

// Live updates ---------------------------------------------------------------

type Listener = (e: { type: string; id?: string; space?: string; status?: string }) => void;
const listeners = new Set<Listener>();
let source: EventSource | null = null;

export function onEvent(fn: Listener) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function connectEvents() {
  if (source) return;
  source = new EventSource('/api/events');
  source.onmessage = (m) => {
    let e: any;
    try {
      e = JSON.parse(m.data);
    } catch {
      return;
    }
    if (e.type === 'notification') refreshUnread();
    for (const l of listeners) l(e);
  };
  refreshUnread();
}

export function disconnectEvents() {
  source?.close();
  source = null;
}

export async function refreshUnread() {
  try {
    const r = await get<{ unread: number }>('/api/notifications/count', { background: true });
    session.unread = r.unread;
  } catch {}
}

setSignedOutHandler(() => {
  if (!session.me) return;
  disconnectEvents();
  session.me = null;
  if (route.path !== '/signin') navigate('/signin?next=' + encodeURIComponent(route.path + route.search), true);
});
