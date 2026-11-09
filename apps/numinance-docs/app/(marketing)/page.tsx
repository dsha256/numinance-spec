import { Sheet } from '@/components/landing/ui/sheet';
import { Hero } from '@/components/landing/sections/hero';
import { System } from '@/components/landing/sections/system';
import { Workflow } from '@/components/landing/sections/workflow';
import { Governance } from '@/components/landing/sections/governance';
import { Architecture } from '@/components/landing/sections/architecture';
import { Evidence } from '@/components/landing/sections/evidence';
import { Developers } from '@/components/landing/sections/developers';
import { Feedback } from '@/components/landing/sections/feedback';
import { Start } from '@/components/landing/sections/start';

/**
 * Nine chapters, one question each:
 * Hero what is it · System why not just code · Workflow how governed work moves (UNDERSTAND → EVIDENCE) ·
 * Governance who may act · Architecture where it runs · Evidence how it is checked ·
 * Open spec + CLI how developers integrate · Feedback who scrutinises it · Start what next.
 */
export default function Home() {
  return (
    <Sheet>
      <Hero />
      <System />
      <Workflow />
      <Governance />
      <Architecture />
      <Evidence />
      <Developers />
      <Feedback />
      <Start />
    </Sheet>
  );
}
