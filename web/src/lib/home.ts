export const HOME_SECTIONS: { key: string; title: string }[] = [
  { key: 'assigned', title: 'Assigned to me' },
  { key: 'approvals', title: 'Awaiting my approval' },
  { key: 'myItems', title: 'My engineering items in Draft or In Review' },
  { key: 'recent', title: 'Recently viewed' },
  { key: 'activity', title: 'Recent activity in my spaces' },
  { key: 'notifications', title: 'Unread notifications' },
];

export function homeLayout(saved: { key: string; on: boolean }[] | undefined) {
  const out: { key: string; on: boolean }[] = [];
  for (const s of saved || []) if (HOME_SECTIONS.some((h) => h.key === s.key) && !out.some((o) => o.key === s.key)) out.push(s);
  for (const h of HOME_SECTIONS) if (!out.some((o) => o.key === h.key)) out.push({ key: h.key, on: true });
  return out;
}
