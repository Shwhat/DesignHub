// Page forms. A template defines fields; the page keeps the layout, the
// fields and the values, and its body is the markdown generated here from
// them. Tokens {{key}} in the layout mark where each field goes. A token on
// a line of its own is written as a block (lists and tables); inside other
// text it is written on one line.

export interface FormField {
  key: string;
  label: string;
  type: string;
  required?: boolean;
  options?: string[];
  default?: string;
  help?: string;
  kinds?: string;
  columns?: FormField[];
}

export interface PageForm {
  layout: string;
  fields: FormField[];
  values: Record<string, any>;
}

export const fieldTypes: { value: string; label: string }[] = [
  { value: 'text', label: 'Text' },
  { value: 'textarea', label: 'Long text' },
  { value: 'date', label: 'Date' },
  { value: 'user', label: 'Person' },
  { value: 'users', label: 'People' },
  { value: 'select', label: 'Dropdown' },
  { value: 'toggle', label: 'Toggle' },
  { value: 'record', label: 'Record' },
  { value: 'list', label: 'List' },
  { value: 'table', label: 'Table' },
];

export const columnTypes: { value: string; label: string }[] = [
  { value: 'index', label: 'Row number' },
  { value: 'text', label: 'Text' },
  { value: 'date', label: 'Date' },
  { value: 'user', label: 'Person' },
  { value: 'select', label: 'Dropdown' },
  { value: 'toggle', label: 'Toggle' },
  { value: 'record', label: 'Record' },
];

export const recordKinds = ['part', 'assembly', 'requirement', 'procedure', 'document', 'page', 'task', 'issue', 'action', 'epic', 'ncr', 'change_order'];

export interface FillContext {
  date: string;
  user: string;
  space: string;
  title?: string;
}

export function today() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

// fill replaces {date}, {user}, {space} and, when given, {title}. Field
// tokens such as {{date}} are left alone.
export function fill(s: string, c: FillContext) {
  const map: Record<string, string> = { date: c.date, user: c.user, space: c.space };
  if (c.title !== undefined) map.title = c.title;
  return s.replace(/(?<!\{)\{(date|user|space|title)\}(?!\})/g, (m, k) => (k in map ? map[k] : m));
}

export function keyFrom(label: string) {
  const k = label.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^[^a-z]+/, '').replace(/-+$/, '').slice(0, 40);
  return k || 'field';
}

function emptyCell(type: string): any {
  if (type === 'toggle') return false;
  if (type === 'record') return null;
  return '';
}

export function emptyRow(f: FormField) {
  const row: Record<string, any> = {};
  for (const c of f.columns || []) if (c.type !== 'index') row[c.key] = c.default ? c.default : emptyCell(c.type);
  return row;
}

// initialValues gives every field its default, with placeholders filled.
export function initialValues(fields: FormField[], c: FillContext) {
  const v: Record<string, any> = {};
  for (const f of fields) {
    const d = f.default ? fill(f.default, c) : '';
    switch (f.type) {
      case 'toggle': v[f.key] = d === 'true' || d === 'Yes'; break;
      case 'users': case 'list': v[f.key] = d ? [d] : []; break;
      case 'record': v[f.key] = null; break;
      case 'table': v[f.key] = [emptyRow(f)]; break;
      default: v[f.key] = d;
    }
  }
  return v;
}

// normalize makes stored values usable by the form, for pages saved before
// a field existed or with an unexpected shape.
export function normalize(fields: FormField[], values: Record<string, any> | null | undefined) {
  const v: Record<string, any> = { ...(values || {}) };
  for (const f of fields) {
    const x = v[f.key];
    switch (f.type) {
      case 'toggle': v[f.key] = !!x; break;
      case 'users': case 'list': v[f.key] = Array.isArray(x) ? x.map((s) => String(s ?? '')) : x ? [String(x)] : []; break;
      case 'record': v[f.key] = x && typeof x === 'object' && x.id ? x : null; break;
      case 'table': v[f.key] = Array.isArray(x) && x.length ? x.map((r) => ({ ...emptyRow(f), ...(r || {}) })) : [emptyRow(f)]; break;
      default: v[f.key] = x == null ? '' : String(x);
    }
  }
  return v;
}

function isEmpty(type: string, x: any): boolean {
  if (x == null) return true;
  if (typeof x === 'string') return x.trim() === '';
  if (typeof x === 'boolean') return !x;
  if (Array.isArray(x)) return x.every((e) => isEmpty('', e));
  if (typeof x === 'object') return type === 'record' || 'id' in x ? isEmpty('', x.id) : Object.values(x).every((e) => isEmpty('', e));
  return false;
}

const rowUsed = (f: FormField, r: any) => (f.columns || []).some((c) => c.type !== 'index' && !isEmpty(c.type, r?.[c.key]));

export interface Missing { key: string; label: string; }

// missing lists required fields and columns without a value. Rules match
// the server (model.PageForm.Missing).
export function missing(fields: FormField[], values: Record<string, any>): Missing[] {
  const out: Missing[] = [];
  for (const f of fields) {
    const v = values[f.key];
    if (f.type !== 'table') {
      if (f.required && isEmpty(f.type, v)) out.push({ key: f.key, label: f.label });
      continue;
    }
    const rows: any[] = Array.isArray(v) ? v : [];
    let filled = 0;
    rows.forEach((r, i) => {
      if (!rowUsed(f, r)) return;
      filled++;
      for (const c of f.columns || []) {
        if (c.required && c.type !== 'index' && isEmpty(c.type, r[c.key])) out.push({ key: `${f.key}.${i}.${c.key}`, label: `${f.label} row ${i + 1}: ${c.label}` });
      }
    });
    if (f.required && filled === 0) out.push({ key: f.key, label: `${f.label} (at least one row)` });
  }
  return out;
}

const oneLine = (s: string) => s.replace(/\s*\n\s*/g, ' ').trim();
const cell = (s: string) => oneLine(s).replaceAll('|', '\\|');

function recordText(x: any) {
  if (!x || !x.id) return '';
  return `[[${x.id}]]${x.title ? ' ' + x.title : ''}`;
}

function inline(f: FormField, x: any): string {
  switch (f.type) {
    case 'toggle': return x ? 'Yes' : 'No';
    case 'record': return recordText(x);
    case 'users': case 'list': return (Array.isArray(x) ? x : []).map((s) => oneLine(String(s))).filter(Boolean).join(', ');
    case 'table': return '';
    default: return oneLine(String(x ?? ''));
  }
}

function tableBlock(f: FormField, x: any): string {
  const cols = f.columns || [];
  const rows = (Array.isArray(x) ? x : []).filter((r) => rowUsed(f, r));
  if (!rows.length) return 'None.';
  const head = '| ' + cols.map((c) => cell(c.label)).join(' | ') + ' |';
  const sep = '|' + cols.map(() => '---').join('|') + '|';
  const body = rows.map((r, i) => '| ' + cols.map((c) => (c.type === 'index' ? String(i + 1) : cell(inline(c, r[c.key])))).join(' | ') + ' |');
  return [head, sep, ...body].join('\n');
}

function block(f: FormField, x: any): string {
  switch (f.type) {
    case 'textarea': return String(x ?? '').trim();
    case 'users': case 'list': {
      const items = (Array.isArray(x) ? x : []).map((s) => oneLine(String(s))).filter(Boolean);
      return items.length ? items.map((s) => '- ' + s).join('\n') : 'None.';
    }
    case 'table': return tableBlock(f, x);
    default: return inline(f, x);
  }
}

const token = /\{\{\s*([a-z][a-z0-9-]*)\s*\}\}/g;

// render generates the page markdown from a layout and values.
export function render(layout: string, fields: FormField[], values: Record<string, any>, title: string): string {
  const byKey = new Map(fields.map((f) => [f.key, f]));
  const lines = fill(layout, { date: '{date}', user: '{user}', space: '{space}', title }).split('\n').map((line) => {
    const only = line.trim().match(/^\{\{\s*([a-z][a-z0-9-]*)\s*\}\}$/);
    if (only) {
      const f = byKey.get(only[1]);
      return f ? block(f, values[f.key]) : line;
    }
    return line.replace(token, (m, k) => {
      const f = byKey.get(k);
      if (!f) return m;
      const s = inline(f, values[f.key]);
      return line.trimStart().startsWith('|') ? cell(s) : s;
    });
  });
  return lines.join('\n').replace(/\n{3,}/g, '\n\n').trim() + '\n';
}
