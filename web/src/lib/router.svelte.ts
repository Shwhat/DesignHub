// Minimal history router.

export const route = $state({ path: location.pathname, search: location.search, query: new URLSearchParams(location.search) });

let guard: (() => boolean) | null = null;

// setLeaveGuard registers a check that runs before leaving the screen
// (unsaved changes). It returns false to stay.
export function setLeaveGuard(fn: (() => boolean) | null) {
  guard = fn;
}

function update() {
  route.path = location.pathname;
  route.search = location.search;
  route.query = new URLSearchParams(location.search);
}

export function navigate(to: string, replace = false) {
  if (guard && !guard()) return;
  guard = null;
  if (replace) history.replaceState(null, '', to);
  else history.pushState(null, '', to);
  update();
  window.scrollTo(0, 0);
}

export function setQuery(params: Record<string, string | null>) {
  const q = new URLSearchParams(location.search);
  for (const [k, v] of Object.entries(params)) {
    if (v === null || v === '') q.delete(k);
    else q.set(k, v);
  }
  const s = q.toString();
  history.replaceState(null, '', location.pathname + (s ? '?' + s : ''));
  update();
}

window.addEventListener('popstate', () => {
  update();
});

// Intercept clicks on internal links.
document.addEventListener('click', (e) => {
  if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
  const a = (e.target as HTMLElement).closest('a');
  if (!a || a.target || a.hasAttribute('download')) return;
  const href = a.getAttribute('href');
  if (!href || !href.startsWith('/') || href.startsWith('/api/')) return;
  e.preventDefault();
  navigate(href);
});

// match returns params when path matches a pattern like /items/:id.
export function match(pattern: string, path: string): Record<string, string> | null {
  const p = pattern.split('/');
  const s = path.split('/');
  if (p.length !== s.length) return null;
  const out: Record<string, string> = {};
  for (let i = 0; i < p.length; i++) {
    if (p[i].startsWith(':')) out[p[i].slice(1)] = decodeURIComponent(s[i]);
    else if (p[i] !== s[i]) return null;
  }
  return out;
}

// recordPath returns the screen for a record id.
export function recordPath(id: string, kind = ''): string {
  if (/^PG-/.test(id) || kind === 'page') return '/pages/' + id;
  if (/^CO-/.test(id) || kind === 'change_order') return '/change-orders/' + id;
  if (/^(TSK|ISS|NCR|ACT|EPC)-/.test(id)) return '/work/' + id;
  if (/^SP-/.test(id)) return '/spaces/' + id;
  return '/items/' + encodeURIComponent(id);
}
