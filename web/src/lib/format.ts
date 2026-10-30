import { session } from './state.svelte';

function tz(): string | undefined {
  return session.me?.timeZone || undefined;
}

export function dateTime(v: string | Date | null | undefined): string {
  if (!v) return '';
  const d = typeof v === 'string' ? new Date(v) : v;
  return d.toLocaleString(undefined, { timeZone: tz(), year: 'numeric', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
}

export function date(v: string | Date | null | undefined): string {
  if (!v) return '';
  if (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(v)) {
    const [y, m, d] = v.split('-').map(Number);
    return new Date(y, m - 1, d).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
  }
  const d = typeof v === 'string' ? new Date(v) : v;
  return d.toLocaleDateString(undefined, { timeZone: tz(), year: 'numeric', month: 'short', day: 'numeric' });
}

export function ago(v: string | null | undefined): string {
  if (!v) return '';
  const s = (Date.now() - new Date(v).getTime()) / 1000;
  if (s < 60) return 'just now';
  if (s < 3600) return Math.floor(s / 60) + ' min ago';
  if (s < 86400) return Math.floor(s / 3600) + ' h ago';
  if (s < 7 * 86400) return Math.floor(s / 86400) + ' d ago';
  return date(v);
}

export function bytes(n: number): string {
  if (n < 1024) return n + ' B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB';
  return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB';
}

export function qty(n: number): string {
  return Number.isInteger(n) ? String(n) : String(Math.round(n * 1000) / 1000);
}

export const kindLabels: Record<string, string> = {
  part: 'Part',
  assembly: 'Assembly',
  requirement: 'Requirement',
  procedure: 'Procedure',
  document: 'Document',
  page: 'Page',
  change_order: 'Change order',
  task: 'Task',
  issue: 'Issue',
  action: 'Action',
  epic: 'Epic',
  ncr: 'Nonconformance',
};

export function kindLabel(k: string) {
  return kindLabels[k] || k;
}

export function today(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
