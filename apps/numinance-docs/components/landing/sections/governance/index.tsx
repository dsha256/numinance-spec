'use client';

import { useState } from 'react';
import { Figure, FigureHeader, FigureLede, StatusPlate } from '@/components/landing/ui/figure';
import { Duties } from './duties';
import { Machine } from './machine';
import type { ScenarioId } from './data';

export function Governance() {
  const [scenario, setScenario] = useState<ScenarioId>('A');
  return (
    <Figure
      id="governance"
      chapter="GOVERNANCE"
      status="governance"
      aria-label="Governance"
      className="px-(--lp-gutter) py-(--lp-section)"
    >
      <FigureHeader lead={<StatusPlate>policy gate</StatusPlate>} title={['Turn autonomy up.\nNever turn ', { term: 'control' }, ' off.']} />
      <FigureLede className="mt-5">
        Choose how far Numinance may go—from observation and suggestions to implementation, merging, releases, and
        operations. Risk, environment, scoped grants, and external policy determine how much authority each action
        actually receives.
      </FigureLede>
      <p className="mt-5 font-pixel text-[17px] leading-6 text-lp-ink">Your policies decide. Not the model.</p>
      <div className="mt-10 md:mt-14">
        <h3 className="font-pixel text-[15px] leading-6">From OBSERVE to OPERATE.</h3>
        <p className="mt-1 max-w-[62ch] text-sm leading-relaxed text-lp-ink-2">
          Full autonomy is configurable, never assumed. Floors, the host sandbox ceiling and scoped grants always hold, and the model never grants itself more.
        </p>
      </div>
      <div className="mt-6 flex flex-col gap-6">
        <Machine onScenario={setScenario} />
        <Duties scenario={scenario} />
      </div>
    </Figure>
  );
}
