// Markdown rendering for previews and page views. Raw HTML is shown as text,
// never rendered. Fenced blocks named after a live block become placeholders
// that the Markdown component fills with live data.
import { unified } from 'unified';
import remarkParse from 'remark-parse';
import remarkGfm from 'remark-gfm';
import remarkRehype from 'remark-rehype';
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize';
import rehypeStringify from 'rehype-stringify';
import { visit } from 'unist-util-visit';
import { recordPath } from './router.svelte';

export const LIVE_BLOCKS = ['bom', 'work', 'item', 'traceability', 'changeorders'] as const;

export const blockLabels: Record<string, string> = {
  bom: 'Bill of Design',
  work: 'Work list',
  item: 'Item card',
  traceability: 'Traceability',
  changeorders: 'Change orders',
};

const idPattern = /\[\[([A-Z]{2,5}\.[A-Za-z0-9/_+.-]+|[0-9]{6}-[0-9]{3}|[0-9]{7}|[A-Z]{2,3}-[0-9]{4,})\]\]/g;

function remarkDesignHub() {
  return (tree: any) => {
    // Raw HTML is displayed as text.
    visit(tree, 'html', (node: any) => {
      node.type = 'text';
    });
    // Live blocks.
    visit(tree, 'code', (node: any, index: number | undefined, parent: any) => {
      const lang = (node.lang || '').toLowerCase();
      if (!(LIVE_BLOCKS as readonly string[]).includes(lang) || index === undefined || !parent) return;
      parent.children[index] = {
        type: 'liveBlock',
        data: { hName: 'div', hProperties: { className: ['live-block'], dataBlock: lang, dataArgs: ((node.value || '') + ' ' + (node.meta || '')).trim() }, hChildren: [] },
      };
    });
    // [[ID]] links to records.
    visit(tree, 'text', (node: any, index: number | undefined, parent: any) => {
      if (index === undefined || !parent || parent.type === 'link') return;
      const value: string = node.value;
      if (!value.includes('[[')) return;
      const parts: any[] = [];
      let last = 0;
      for (const m of value.matchAll(idPattern)) {
        if (m.index! > last) parts.push({ type: 'text', value: value.slice(last, m.index) });
        parts.push({ type: 'link', url: recordPath(m[1]), children: [{ type: 'text', value: m[1] }], data: { hProperties: { className: ['record-link'] } } });
        last = m.index! + m[0].length;
      }
      if (parts.length === 0) return;
      if (last < value.length) parts.push({ type: 'text', value: value.slice(last) });
      parent.children.splice(index, 1, ...parts);
      return index + parts.length;
    });
  };
}

const schema = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    div: [...(defaultSchema.attributes?.div || []), 'dataBlock', 'dataArgs', ['className', 'live-block']],
    a: [...(defaultSchema.attributes?.a || []), ['className', 'record-link']],
    code: [...(defaultSchema.attributes?.code || []), ['className', /^language-./]],
    input: [...(defaultSchema.attributes?.input || []), 'checked', 'disabled', 'type'],
  },
  protocols: { ...defaultSchema.protocols, src: ['http', 'https'], href: ['http', 'https', 'mailto'] },
};

const processor = unified()
  .use(remarkParse)
  .use(remarkGfm)
  .use(remarkDesignHub)
  .use(remarkRehype)
  .use(rehypeSanitize, schema as any)
  .use(rehypeStringify);

export function renderMarkdown(md: string): string {
  try {
    return String(processor.processSync(md || ''));
  } catch (e) {
    const div = document.createElement('div');
    div.textContent = md;
    return '<pre>' + div.innerHTML + '</pre>';
  }
}

// parseArgs reads "key=value" pairs and a leading id from a block's arguments.
export function parseArgs(args: string): { id: string; params: Record<string, string> } {
  const params: Record<string, string> = {};
  let id = '';
  for (const tok of args.split(/\s+/).filter(Boolean)) {
    const eq = tok.indexOf('=');
    if (eq > 0) params[tok.slice(0, eq)] = tok.slice(eq + 1);
    else if (!id) id = tok;
  }
  return { id, params };
}

// blockMarkdown returns the markdown for a new live block.
export function blockMarkdown(kind: string, args: string): string {
  return '\n```' + kind + '\n' + args + '\n```\n';
}
