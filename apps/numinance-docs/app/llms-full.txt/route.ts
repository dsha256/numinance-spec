import { source } from '@/lib/source';
import { siteConfig } from '@/lib/site-config';

export const revalidate = false;

/** Full public documentation text for machine readers. Only content/docs is included; no private design text. */
export async function GET() {
  const base = siteConfig.url.replace(/\/$/, '');
  const pages = [...source.getPages()].sort((a, b) => a.url.localeCompare(b.url));
  const parts = await Promise.all(
    pages.map(async (p) => `# ${p.data.title}\nURL: ${base}${p.url}\n\n${p.data.description ? `${p.data.description}\n\n` : ''}${await p.data.getText('processed')}`),
  );
  return new Response(parts.join('\n\n---\n\n'), { headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
}
