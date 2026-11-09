import { source } from '@/lib/source';
import { siteConfig } from '@/lib/site-config';

export const revalidate = false;

export function GET() {
  const base = siteConfig.url.replace(/\/$/, '');
  const pages = [...source.getPages()].sort((a, b) => a.url.localeCompare(b.url));
  const lines = [
    '# Numinance',
    '',
    '> Numinance is building an AI software engineer that understands your systems, works within configurable autonomy boundaries, and produces independently verifiable evidence. Pre-alpha. The engine is proprietary; the numinance-spec contract (Apache-2.0) is open: https://github.com/dsha256/numinance-spec',
    '',
    'Full text: /llms-full.txt. Any page as Markdown: append .md to its /docs URL.',
    '',
    '## Documentation',
    '',
    ...pages.map((p) => `- [${p.data.title}](${base}${p.url})${p.data.description ? `: ${p.data.description}` : ''}`),
    '',
  ];
  return new Response(lines.join('\n'), {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
}
