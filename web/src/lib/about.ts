// The About guide is a set of Markdown files in src/about, bundled with each
// build. A file named "NN-slug.md" becomes the page /about/slug, ordered by
// NN, titled by its first "# " heading.

export type AboutPage = { slug: string; title: string; body: string };

const files = import.meta.glob('../about/*.md', { query: '?raw', import: 'default', eager: true }) as Record<string, string>;

export const aboutPages: AboutPage[] = Object.keys(files)
  .sort()
  .map((path) => {
    const body = files[path];
    const slug = path.replace(/^.*\/\d+-/, '').replace(/\.md$/, '');
    const title = body.match(/^# (.+)$/m)?.[1].trim() || slug;
    return { slug, title, body };
  });
