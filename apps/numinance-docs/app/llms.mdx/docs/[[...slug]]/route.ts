import { source } from '@/lib/source';
import { notFound } from 'next/navigation';

export const revalidate = false;

/** Markdown export of a docs page (`/docs/<slug>.md`, rewritten here by next.config.mjs). */
export async function GET(_req: Request, { params }: { params: Promise<{ slug?: string[] }> }) {
  const { slug } = await params;
  const page = source.getPage(slug);
  if (!page) notFound();
  const text = await page.data.getText('processed');
  return new Response(`# ${page.data.title}\n\n${page.data.description ? `${page.data.description}\n\n` : ''}${text}\n`, {
    headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
  });
}

export function generateStaticParams() {
  return source.generateParams();
}
