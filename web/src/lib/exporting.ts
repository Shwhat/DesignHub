// CSV and PDF export helpers. Table exports are built from the rows on
// screen and reported to the server for the audit log. PDF exports send the
// record as shown, with styles and images inlined, to the server.
import { post, ApiError, csrfToken } from './api';

export type Column<T> = { label: string; value: (row: T) => any };

function cell(v: any): string {
  if (v === null || v === undefined) return '';
  let s = Array.isArray(v) ? v.join('; ') : String(v);
  // Stop spreadsheet programs from treating text as a formula.
  if (/^[=+\-@\t\r]/.test(s) && isNaN(Number(s))) s = "'" + s;
  return /[",\n\r]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s;
}

export function saveBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 2000);
}

/** exportCSV downloads rows as a CSV file and records the export. */
export function exportCSV<T>(view: string, columns: Column<T>[], rows: T[]) {
  const lines = [columns.map((c) => cell(c.label)).join(',')];
  for (const r of rows) lines.push(columns.map((c) => cell(c.value(r))).join(','));
  const blob = new Blob(['﻿' + lines.join('\r\n') + '\r\n'], { type: 'text/csv;charset=utf-8' });
  const stamp = new Date().toISOString().slice(0, 10);
  saveBlob(blob, `designhub-${view.toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${stamp}.csv`);
  post('/api/exports/log', { view, rows: rows.length, kind: 'csv' }).catch(() => {});
}

/** fullExport downloads the server's all-fields export of a record type. */
export function fullExport(type: string, query = '') {
  const a = document.createElement('a');
  a.href = `/api/exports/${type}${query ? '?' + query : ''}`;
  a.download = '';
  document.body.appendChild(a);
  a.click();
  a.remove();
}

// ---------------------------------------------------------------------------
// PDF

async function inlineImages(root: HTMLElement) {
  const imgs = Array.from(root.querySelectorAll('img'));
  await Promise.all(
    imgs.map(async (img) => {
      const src = img.getAttribute('src') || '';
      if (!src || src.startsWith('data:')) return;
      try {
        const res = await fetch(src, { credentials: 'same-origin' });
        if (!res.ok) throw new Error();
        const b = await res.blob();
        const data = await new Promise<string>((ok) => {
          const fr = new FileReader();
          fr.onload = () => ok(String(fr.result));
          fr.readAsDataURL(b);
        });
        img.setAttribute('src', data);
      } catch {
        img.replaceWith(Object.assign(document.createElement('span'), { textContent: '[image not available]' }));
      }
    }),
  );
}

function pageStyles(): string {
  let css = '';
  for (const sheet of Array.from(document.styleSheets)) {
    try {
      for (const rule of Array.from(sheet.cssRules)) css += rule.cssText + '\n';
    } catch {}
  }
  // Print in the light theme, without interactive chrome.
  css += `body{background:#fff!important;margin:0}.no-print,button,.tabs,.editor-toolbar,.decision-bar,.comment-marker{display:none!important}
    .card{break-inside:avoid}.pdf-break{break-before:page}table{break-inside:auto}tr{break-inside:avoid}`;
  return css;
}

/** pdfHTML clones elements into one standalone HTML document. */
export async function pdfHTML(parts: (HTMLElement | string)[]): Promise<string> {
  const wrap = document.createElement('div');
  parts.forEach((p, i) => {
    const sec = document.createElement('section');
    if (i > 0) sec.className = 'pdf-break';
    if (typeof p === 'string') sec.innerHTML = p;
    else sec.appendChild(p.cloneNode(true));
    wrap.appendChild(sec);
  });
  wrap.querySelectorAll('script, iframe, .no-print').forEach((e) => e.remove());
  await inlineImages(wrap);
  return `<!doctype html><html data-theme="light"><head><meta charset="utf-8"><style>${pageStyles()}</style></head><body><div class="content" style="max-width:none;padding:0 2px">${wrap.innerHTML}</div></body></html>`;
}

export function paper(): string {
  try {
    return localStorage.getItem('dh.paper') || (navigator.language === 'en-US' ? 'letter' : 'a4');
  } catch {
    return 'letter';
  }
}

export function setPaper(p: string) {
  try {
    localStorage.setItem('dh.paper', p);
  } catch {}
}

/** exportPDF sends HTML to the server and downloads the PDF. */
export async function exportPDF(id: string, html: string, scope: string, pages = 1, size = paper()) {
  const res = await fetch('/api/pdf', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken() },
    credentials: 'same-origin',
    body: JSON.stringify({ id, html, paper: size, scope, pages }),
  });
  if (!res.ok) {
    let msg = 'The PDF could not be created.';
    try {
      msg = (await res.json()).error || msg;
    } catch {}
    throw new ApiError(res.status, msg);
  }
  saveBlob(await res.blob(), id + '.pdf');
}
