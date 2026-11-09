import type { MetadataRoute } from 'next';
import { siteConfig } from '@/lib/site-config';
import { source } from '@/lib/source';

const routes = ['', '/architecture', '/protocol', '/evidence', '/integrations', '/changelog'];

export default function sitemap(): MetadataRoute.Sitemap {
  const docs = source.getPages().map((p) => p.url);
  return [...routes, ...docs].map((r) => ({ url: `${siteConfig.url}${r}` }));
}
