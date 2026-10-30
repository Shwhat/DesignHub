// API client: JSON requests with the session's CSRF token.

export class ApiError extends Error {
  status: number;
  code: string;
  data: any;
  constructor(status: number, message: string, code = '', data: any = null) {
    super(message);
    this.status = status;
    this.code = code;
    this.data = data;
  }
}

let csrf = '';
let onSignedOut: () => void = () => {};

export function setCsrf(token: string) {
  csrf = token;
}

export function setSignedOutHandler(fn: () => void) {
  onSignedOut = fn;
}

export interface Options {
  background?: boolean;
  signal?: AbortSignal;
}

export async function api<T = any>(method: string, path: string, body?: any, opts: Options = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (body !== undefined && !(body instanceof Blob)) headers['Content-Type'] = 'application/json';
  if (method !== 'GET' && csrf) headers['X-CSRF-Token'] = csrf;
  if (opts.background) headers['X-Background'] = '1';
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers,
      body: body === undefined ? undefined : body instanceof Blob ? body : JSON.stringify(body),
      credentials: 'same-origin',
      signal: opts.signal,
    });
  } catch (e: any) {
    if (e?.name === 'AbortError') throw e;
    throw new ApiError(0, 'DesignHub cannot be reached. Check the network connection.');
  }
  const text = await res.text();
  let data: any = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = null;
  }
  if (!res.ok) {
    if (res.status === 401 && path !== '/api/signin') onSignedOut();
    throw new ApiError(res.status, data?.error || res.statusText || 'Request failed', data?.code || '', data?.data ?? null);
  }
  return data as T;
}

export const get = <T = any>(path: string, opts?: Options) => api<T>('GET', path, undefined, opts);
export const post = <T = any>(path: string, body: any = {}) => api<T>('POST', path, body);
export const put = <T = any>(path: string, body: any) => api<T>('PUT', path, body);
export const patch = <T = any>(path: string, body: any) => api<T>('PATCH', path, body);
export const del = <T = any>(path: string) => api<T>('DELETE', path);

export interface UploadedFile {
  sha256: string;
  name: string;
  size: number;
  mime: string;
  role?: string;
}

export async function upload(file: File): Promise<UploadedFile> {
  const res = await fetch('/api/files?name=' + encodeURIComponent(file.name), {
    method: 'POST',
    headers: { 'X-CSRF-Token': csrf, 'Content-Type': file.type || 'application/octet-stream' },
    body: file,
    credentials: 'same-origin',
  });
  const data = await res.json().catch(() => null);
  if (!res.ok) throw new ApiError(res.status, data?.error || 'Upload failed');
  return data;
}

export function fileURL(record: string, f: { sha256: string; name: string }, inline = false) {
  return `/api/records/${encodeURIComponent(record)}/files/${f.sha256}?name=${encodeURIComponent(f.name)}${inline ? '&inline=1' : ''}`;
}
