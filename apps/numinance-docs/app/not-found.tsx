import Link from 'next/link';
import { Symbol } from '@/components/brand/logo';

export default function NotFound() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-6 px-6 text-center">
      <Symbol className="size-12 text-nm-faint" />
      <p className="nm-eyebrow">404 · No such record</p>
      <h1 className="text-4xl font-semibold tracking-tight">This page isn&apos;t in the chain.</h1>
      <Link href="/" className="rounded-full bg-nm-fg px-5 py-2.5 text-sm font-medium text-nm-bg">Return home</Link>
    </main>
  );
}
