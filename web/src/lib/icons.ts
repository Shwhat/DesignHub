// Interface icons. Hand-drawn inline SVG on a 20 x 20 view box, outlines in
// dark slate with small tinted fills that carry meaning. Drawn in light theme
// colors; the dark theme inverts them (see .ico in app.css).

const S = '#2b3442';
const G = '#556';
const svg = (body: string) => `<svg viewBox="0 0 20 20" aria-hidden="true">${body}</svg>`;

export const ICONS: Record<string, string> = {
  // Navigation
  home: svg(`<path d="M3 9.5L10 3l7 6.5" fill="none" stroke="${S}" stroke-width="1.5"/><path d="M5 8.5V17h10V8.5" fill="#dbe6f7" stroke="${S}" stroke-width="1.3"/><rect x="8.5" y="12" width="3" height="5" fill="#fff" stroke="${S}"/>`),
  mywork: svg(`<rect x="3" y="2.5" width="14" height="15" rx="1" fill="#fff" stroke="${G}" stroke-width="1.2"/><path d="M5.5 6.5l1.3 1.3 2.2-2.4M5.5 11.5l1.3 1.3 2.2-2.4" fill="none" stroke="#2f7d32" stroke-width="1.5"/><path d="M11 7h4M11 12h4M6 15.5h9" stroke="${G}" stroke-width="1.2"/>`),
  library: svg(`<rect x="3" y="3" width="4" height="14" fill="#dbe6f7" stroke="#2b4f91"/><rect x="8" y="3" width="4" height="14" fill="#f4e2c2" stroke="#8a6d1d"/><path d="M13 4l3.5-.8 2.5 13-3.5.8z" fill="#e3f0e3" stroke="#2f7d32"/>`),
  co: svg(`<path d="M4 2h8l4 4v12H4z" fill="#fff" stroke="${G}" stroke-width="1.2"/><path d="M12 2v4h4" fill="none" stroke="${G}" stroke-width="1.2"/><path d="M6.5 10h6l-2-2M13.5 14h-6l2 2" fill="none" stroke="#1f5fbf" stroke-width="1.4"/>`),
  search: svg(`<circle cx="8.5" cy="8.5" r="5" fill="#fff" stroke="${S}" stroke-width="1.6"/><path d="M12.2 12.2l4.8 4.8" stroke="${S}" stroke-width="2"/>`),
  bell: svg(`<path d="M5 14V9a5 5 0 0110 0v5l1.5 1.5h-13z" fill="#f4d58d" stroke="#8a6d1d" stroke-width="1.2"/><path d="M8.5 17a1.6 1.6 0 003 0" fill="none" stroke="#8a6d1d" stroke-width="1.2"/>`),
  about: svg(`<circle cx="10" cy="10" r="7.5" fill="#dbe6f7" stroke="#2b4f91" stroke-width="1.2"/><path d="M10 9v5.5" stroke="#2b4f91" stroke-width="1.8"/><circle cx="10" cy="6.2" r="1.1" fill="#2b4f91"/>`),
  space: svg(`<path d="M2 5h6l2 2h8v9H2z" fill="#f4d58d" stroke="#8a6d1d" stroke-width="1.1"/>`),
  board: svg(`<rect x="2.5" y="3" width="15" height="14" rx="1" fill="#fff" stroke="${G}" stroke-width="1.2"/><path d="M7.5 3v14M12.5 3v14" stroke="${G}"/><rect x="3.8" y="5" width="2.4" height="2.5" fill="#dbe6f7"/><rect x="8.8" y="5" width="2.4" height="2.5" fill="#f4e2c2"/><rect x="8.8" y="9" width="2.4" height="2.5" fill="#f4e2c2"/><rect x="13.8" y="5" width="2.4" height="2.5" fill="#e3f0e3"/>`),
  list: svg(`<path d="M7 5h10M7 10h10M7 15h10" stroke="${S}" stroke-width="1.5"/><circle cx="4" cy="5" r="1.2" fill="${S}"/><circle cx="4" cy="10" r="1.2" fill="${S}"/><circle cx="4" cy="15" r="1.2" fill="${S}"/>`),
  ncr: svg(`<path d="M10 2.5l8 14H2z" fill="#fde7c7" stroke="#b25d00" stroke-width="1.3"/><path d="M10 7.5v4.5" stroke="#b25d00" stroke-width="1.8"/><circle cx="10" cy="14.3" r="1" fill="#b25d00"/>`),
  trace: svg(`<rect x="2" y="3" width="6" height="4" fill="#dbe6f7" stroke="#2b4f91"/><rect x="12" y="8" width="6" height="4" fill="#f4e2c2" stroke="#8a6d1d"/><rect x="2" y="13" width="6" height="4" fill="#e3f0e3" stroke="#2f7d32"/><path d="M8 5h2v10H8M10 10h2" fill="none" stroke="${S}" stroke-width="1.2"/>`),
  baseline: svg(`<path d="M3 3h7l7 7-7 7-7-7z" fill="#e3f0e3" stroke="#2f7d32" stroke-width="1.2"/><circle cx="7" cy="7" r="1.5" fill="#fff" stroke="#2f7d32"/>`),
  page: svg(`<path d="M5 2h7l4 4v12H5z" fill="#fff" stroke="${G}" stroke-width="1.2"/><path d="M12 2v4h4" fill="none" stroke="${G}" stroke-width="1.2"/><path d="M7.5 10h6M7.5 13h6" stroke="${G}"/>`),
  settings: svg(`<circle cx="10" cy="10" r="3" fill="none" stroke="${S}" stroke-width="1.6"/><path d="M10 2v3M10 15v3M2 10h3M15 10h3M4.3 4.3l2.1 2.1M13.6 13.6l2.1 2.1M4.3 15.7l2.1-2.1M13.6 6.4l2.1-2.1" stroke="${S}" stroke-width="1.6"/>`),
  menu: svg(`<path d="M3 5h14M3 10h14M3 15h14" stroke="${S}" stroke-width="1.8"/>`),

  // Actions
  plus: svg(`<path d="M10 4v12M4 10h12" stroke="#2f7d32" stroke-width="2"/>`),
  close: svg(`<path d="M5 5l10 10M15 5L5 15" stroke="${S}" stroke-width="1.6"/>`),
  remove: svg(`<path d="M6 6l8 8M14 6l-8 8" stroke="#a33" stroke-width="1.6"/>`),
  more: svg(`<circle cx="4.5" cy="10" r="1.5" fill="${S}"/><circle cx="10" cy="10" r="1.5" fill="${S}"/><circle cx="15.5" cy="10" r="1.5" fill="${S}"/>`),
  right: svg(`<path d="M7.5 5l5 5-5 5" fill="none" stroke="${S}" stroke-width="1.6"/>`),
  down: svg(`<path d="M5 7.5l5 5 5-5" fill="none" stroke="${S}" stroke-width="1.6"/>`),
  caret: svg(`<path d="M5.5 8l4.5 4.5L14.5 8z" fill="${S}"/>`),
  up: svg(`<path d="M10 16V4M5 9l5-5 5 5" fill="none" stroke="${S}" stroke-width="1.6"/>`),
  dn: svg(`<path d="M10 4v12M5 11l5 5 5-5" fill="none" stroke="${S}" stroke-width="1.6"/>`),
  sortup: svg(`<path d="M5.5 12.5L10 7l4.5 5.5z" fill="${S}"/>`),
  sortdn: svg(`<path d="M5.5 7.5L10 13l4.5-5.5z" fill="${S}"/>`),
  grip: svg(`<g fill="#7a8899"><circle cx="7.5" cy="5" r="1.3"/><circle cx="12.5" cy="5" r="1.3"/><circle cx="7.5" cy="10" r="1.3"/><circle cx="12.5" cy="10" r="1.3"/><circle cx="7.5" cy="15" r="1.3"/><circle cx="12.5" cy="15" r="1.3"/></g>`),
  check: svg(`<path d="M4.5 10.5l3.5 3.5 7.5-8" fill="none" stroke="#2f7d32" stroke-width="2"/>`),
  lock: svg(`<rect x="4" y="9" width="12" height="9" rx="1" fill="${S}"/><path d="M7 9V6a3 3 0 016 0v3" fill="none" stroke="${S}" stroke-width="1.6"/>`),
  signout: svg(`<path d="M11 3H4v14h7" fill="none" stroke="${G}" stroke-width="1.3"/><path d="M8 10h9M14 7l3 3-3 3" fill="none" stroke="#a33" stroke-width="1.5"/>`),
  admin: svg(`<path d="M10 2l7 3v5c0 4-3 7-7 8-4-1-7-4-7-8V5z" fill="#dbe6f7" stroke="#2b4f91" stroke-width="1.2"/><path d="M7 10l2 2 4-4" fill="none" stroke="#2b4f91" stroke-width="1.5"/>`),
  user: svg(`<circle cx="10" cy="7" r="3.5" fill="#dbe6f7" stroke="#2b4f91" stroke-width="1.2"/><path d="M3.5 17.5a6.5 6.5 0 0113 0" fill="#dbe6f7" stroke="#2b4f91" stroke-width="1.2"/>`),

  // Work item kinds
  task: svg(`<rect x="3" y="3" width="14" height="14" rx="2" fill="#dbe6f7" stroke="#2b4f91" stroke-width="1.2"/><path d="M6.5 10l2.5 2.5 4.5-5" fill="none" stroke="#2b4f91" stroke-width="1.7"/>`),
  issue: svg(`<circle cx="10" cy="10" r="7" fill="#fbe7e5" stroke="#b3261e" stroke-width="1.3"/><circle cx="10" cy="10" r="2.2" fill="#b3261e"/>`),
  action: svg(`<path d="M3 8h8V4.5l6 5.5-6 5.5V12H3z" fill="#e3f0e3" stroke="#2f7d32" stroke-width="1.2"/>`),
  epic: svg(`<path d="M10 2.5l5.5 7.5-5.5 7.5L4.5 10z" fill="#e7dcf7" stroke="#5b3a91" stroke-width="1.2"/>`),
};

export type IconName = keyof typeof ICONS;
