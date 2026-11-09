import Link from 'next/link';
import { Figure, FigureHeader, FigureLede, StatusPlate } from '@/components/landing/ui/figure';
import { Stage } from './stage';

export function Architecture() {
  return (
    <Figure
      id="architecture"
      chapter="ARCHITECTURE"
      status="architecture · customer boundary"
      aria-label="Architecture"
      className="px-(--lp-gutter) py-(--lp-section)"
    >
      <div className="flex flex-col gap-5">
        <FigureHeader
          lead={<StatusPlate>customer-controlled execution</StatusPlate>}
          title={[{ term: 'Your infrastructure' }, '.\nNot our playground.']}
        />
        <FigureLede>
          Numinance is designed to put the engine, runners, credentials and execution behind your boundary. The model can propose work; the fixed trust kernel decides what may happen. Your release pipeline deploys. Your systems keep the evidence.
        </FigureLede>
        <p className="font-pixel text-[15px] leading-5 text-lp-ink">Intelligence is not authority. Replace the worker. Keep the controls.</p>
      </div>

      <Stage />

      <div className="mt-8 flex flex-wrap gap-x-6 gap-y-2 font-pixel text-[13px]">
        <Link href="/architecture" className="underline decoration-lp-line-strong underline-offset-4 hover:text-lp-orange">
          Architecture overview →
        </Link>
        <Link href="/docs/architecture" className="underline decoration-lp-line-strong underline-offset-4 hover:text-lp-orange">
          /docs/architecture →
        </Link>
      </div>
    </Figure>
  );
}
