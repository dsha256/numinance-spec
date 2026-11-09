import { createMDX } from 'fumadocs-mdx/next';
import { readFileSync } from 'node:fs';

const withMDX = createMDX();

/** Legacy docs routes -> current routes (see lib/docs/redirects.json). */
const legacy = JSON.parse(readFileSync(new URL('./lib/docs/redirects.json', import.meta.url), 'utf8'));

/** @type {import('next').NextConfig} */
const config = {
  reactStrictMode: true,
  async redirects() {
    return Object.entries(legacy).map(([source, destination]) => ({ source, destination, permanent: true }));
  },
  async rewrites() {
    // `/docs/<page>.md` serves the page as Markdown (see app/llms.mdx).
    return [{ source: '/docs/:path*.md', destination: '/llms.mdx/docs/:path*' }];
  },
};

export default withMDX(config);
