import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { Logo } from '@/components/brand/logo';
import { source } from '@/lib/source';
import { SPEC_REPO } from '@/lib/docs/spec';

export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <DocsLayout
      tree={source.getPageTree()}
      githubUrl={SPEC_REPO}
      nav={{ title: <Logo className="h-5" />, url: '/' }}
      sidebar={{ defaultOpenLevel: 0 }}
    >
      {children}
    </DocsLayout>
  );
}
