#!/usr/bin/env node
/**
 * Docs validator: MDX compiles, frontmatter, registered components, meta.json entries,
 * internal links + #anchors, redirects, and a few truthfulness lint rules.
 * Usage: node scripts/validate-docs.mjs [path-prefix-under-content/docs ...]
 * Exit 1 on any error. Warnings never fail.
 */
import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs';
import { join, relative, dirname } from 'node:path';
import { compile } from '@mdx-js/mdx';
import GithubSlugger from 'github-slugger';
import { parse as parseYaml } from 'yaml';

const ROOT = new URL('..', import.meta.url).pathname;
const DOCS = join(ROOT, 'content/docs');
const prefixes = process.argv.slice(2);

const walk = (d) => readdirSync(d).flatMap((f) => {
  const p = join(d, f);
  return statSync(p).isDirectory() ? walk(p) : [p];
});
const all = walk(DOCS);
const mdx = all.filter((f) => f.endsWith('.mdx'));
const urlOf = (f) => {
  let r = relative(DOCS, f).replace(/\.mdx$/, '').replace(/\/index$/, '').replace(/^index$/, '');
  return '/docs' + (r ? '/' + r : '');
};
const errors = [];
const warns = [];
const err = (f, m) => errors.push(`${relative(ROOT, f)}: ${m}`);
const warn = (f, m) => warns.push(`${relative(ROOT, f)}: ${m}`);

const stripFences = (s) => s.replace(/^(```|~~~)[^\n]*\n[\s\S]*?^\1[ \t]*$/gm, (m) => m.replace(/[^\n]/g, ' '));
const anchorsOf = (src) => {
  const slugger = new GithubSlugger();
  const set = new Set();
  for (const line of stripFences(src).split('\n')) {
    const m = /^#{1,6}\s+(.*?)\s*$/.exec(line);
    if (!m) continue;
    const custom = /\[#([\w-]+)\]\s*$/.exec(m[1]);
    if (custom) { set.add(custom[1]); continue; }
    const text = m[1].replace(/`/g, '').replace(/<[^>]+>/g, '').replace(/\[([^\]]+)\]\([^)]*\)/g, '$1').replace(/[*_]/g, '');
    set.add(slugger.slug(text));
  }
  return set;
};

const pages = new Map(); // url -> {file, anchors}
for (const f of mdx) pages.set(urlOf(f), { file: f, src: readFileSync(f, 'utf8') });
for (const p of pages.values()) p.anchors = anchorsOf(p.src);

const redirects = JSON.parse(readFileSync(join(ROOT, 'lib/docs/redirects.json'), 'utf8'));
const known = new Set(['Cards', 'Card', 'Callout', ...(readFileSync(join(ROOT, 'components/docs/mdx.tsx'), 'utf8').match(/\b[A-Z][A-Za-z]+\b/g) ?? [])]);

for (const [url, { file, src }] of pages) {
  if (prefixes.length && !prefixes.some((p) => relative(DOCS, file).startsWith(p))) continue;
  const fm = /^---\n([\s\S]*?)\n---/.exec(src);
  if (!fm) err(file, 'missing frontmatter');
  else {
    try { parseYaml(fm[1]); } catch (e) { err(file, `frontmatter YAML: ${e.message.split('\n')[0]} (quote the value)`); }
    if (!/^title:\s*\S/m.test(fm[1])) err(file, 'frontmatter needs title');
    if (!/^description:\s*\S/m.test(fm[1])) err(file, 'frontmatter needs description');
  }
  try { await compile(src, { development: false }); } catch (e) { err(file, `MDX compile: ${e.message.split('\n')[0]} (${e.line ?? '?'}:${e.column ?? '?'})`); }

  const body = stripFences(src.replace(/^---\n[\s\S]*?\n---/, ''));
  for (const m of body.matchAll(/<([A-Z][A-Za-z0-9]*)\b/g)) if (!known.has(m[1])) err(file, `unregistered component <${m[1]}>`);

  const links = [...body.matchAll(/\]\(([^)\s]+)\)/g)].map((m) => m[1]).concat([...body.matchAll(/href=["']([^"']+)["']/g)].map((m) => m[1]));
  for (const l of links) {
    if (/^(https?:|mailto:)/.test(l)) continue;
    if (l.startsWith('#')) { if (!pages.get(url).anchors.has(l.slice(1))) err(file, `broken anchor ${l}`); continue; }
    if (!l.startsWith('/docs')) { if (l.startsWith('/')) continue; err(file, `relative link not allowed (use absolute /docs/...): ${l}`); continue; }
    const [path, hash] = l.split('#');
    const clean = path.replace(/\/$/, '') || '/docs';
    const target = redirects[clean] ?? clean;
    if (redirects[clean]) warn(file, `link uses legacy route ${clean} -> ${target}`);
    const t = pages.get(target.split('#')[0]);
    if (!t) { err(file, `broken link ${l}`); continue; }
    if (hash && !t.anchors.has(hash)) err(file, `broken anchor ${l}`);
  }

  // truthfulness lint
  const rules = [
    [/brew (install|tap)\b/i, 'brew command: Homebrew is planned, not published'],
    [/curl[^\n|]*\|\s*(ba)?sh/i, 'curl | sh installer'],
    [/npm (i|install) [^\n]*@numinance/i, 'npm install of @numinance/*: not published'],
    [/pip install [^\n]*numinance/i, 'pip install numinance: not published'],
    [/cargo add [^\n]*numinance|crates\.io\/crates\/numinance/i, 'Rust crate claim'],
    [/<artifactId>[^<]*numinance|implementation ['"]dev\.numinance/i, 'Java coordinates claim'],
    [/numi run\b/i, '`numi run` is not in the open build: only allowed inside a planned/designed context'],
    [/docs\.google\.com/i, 'link to private design doc'],
    [/dsha256\/numinance(?!-)\b/, 'link/reference to private repository dsha256/numinance'],
    [/\/Users\/dsha256|docs\/design\/|docs\/adr\//, 'private path or local home path'],
    [/world'?s first/i, 'unverified "world\'s first" claim'],
    [/self[- ]?heal/i, '"self-heal(ing)": must be described as bounded repair'],
  ];
  const raw = src.replace(/^---\n[\s\S]*?\n---/, '');
  for (const [re, msg] of rules) if (re.test(raw)) warn(file, `lint: ${msg}`);
}

// meta.json entries
for (const f of all.filter((x) => x.endsWith('meta.json'))) {
  const dir = dirname(f);
  const meta = JSON.parse(readFileSync(f, 'utf8'));
  for (const e of meta.pages ?? []) {
    if (e.startsWith('---') || e.startsWith('...')) continue;
    const name = e.replace(/^\.\.\./, '');
    if (!existsSync(join(dir, name + '.mdx')) && !existsSync(join(dir, name, 'meta.json')) && !existsSync(join(dir, name, 'index.mdx'))) {
      if (!prefixes.length || prefixes.some((p) => relative(DOCS, dir).startsWith(p) || p.startsWith(relative(DOCS, dir)))) err(f, `lists missing page "${e}"`);
    }
  }
  if (!prefixes.length) {
    const listed = new Set((meta.pages ?? []).map((e) => e.replace(/^\.\.\./, '')));
    for (const x of readdirSync(dir)) {
      if (x.endsWith('.mdx') && !listed.has(x.replace(/\.mdx$/, ''))) warn(f, `page "${x}" not listed in meta.json`);
    }
  }
}
for (const [from, to] of Object.entries(redirects)) if (!pages.has(to)) err(join(ROOT, 'lib/docs/redirects.json'), `redirect ${from} -> missing ${to}`);

for (const w of warns) console.log('warn  ', w);
for (const e of errors) console.log('ERROR ', e);
console.log(`\n${pages.size} pages, ${errors.length} errors, ${warns.length} warnings`);
process.exit(errors.length ? 1 : 0);
