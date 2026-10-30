import { get } from './api';

export const lists = $state({ loaded: false, values: {} as Record<string, string[]>, manufacturers: [] as { code: string; name: string; active: boolean }[] });

export async function loadLists(force = false) {
  if (lists.loaded && !force) return;
  try {
    const ls: any[] = await get('/api/lists', { background: true });
    const v: Record<string, string[]> = {};
    for (const l of ls) v[l.key] = l.values.filter((x: any) => x.active).map((x: any) => x.value);
    lists.values = v;
    lists.manufacturers = await get('/api/manufacturers', { background: true });
    lists.loaded = true;
  } catch {}
}

export function listOf(key: string): string[] {
  return lists.values[key] || [];
}
