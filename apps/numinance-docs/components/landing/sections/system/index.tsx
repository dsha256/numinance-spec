import {
  Figure,
  FigureHeader,
  FigureLede,
} from "@/components/landing/ui/figure";
import { Ruler } from "@/components/landing/ui/sheet";
import { SystemMachine } from "./machine";

const NOTES = [
  {
    n: "01",
    k: "CONTEXT FIRST",
    t: "Architecture. Dependencies. Conventions. Constraints. Understand what already exists before deciding what should change.",
  },
  {
    n: "02",
    k: "AUTONOMY BY DESIGN",
    t: "From suggestions to independent execution, the level of responsibility is configurable—not hard-coded into a chat session.",
  },
  {
    n: "03",
    k: "ACCOUNTABLE EXECUTION",
    t: "Every permitted action belongs to a governed workflow, from plan to release, observation and recovery. Verification and evidence are part of the engineering process.",
  },
] as const;

export function System() {
  return (
    <Figure
      id="system"
      chapter="SYSTEM"
      status="engineering, not generation"
      aria-label="Code generation is not the whole engineering job"
      className="px-(--lp-gutter) py-(--lp-section)"
    >
      <FigureHeader
        className="max-w-none! text-[clamp(1.85rem,0.4rem+6.2vw,6.25rem)]! leading-[0.95]!"
        title={["Code generation was never\nthe ", { term: "whole job" }, "."]}
      />
      <div className="mt-8 grid gap-4 md:mt-12 md:grid-cols-12">
        <FigureLede className="md:col-span-6 md:col-start-7">
          Generating code is one thing. Understanding a system, navigating its
          dependencies, respecting its constraints, and taking responsibility
          for the outcome is engineering.
        </FigureLede>
        <p className="font-pixel text-lg leading-snug text-lp-ink md:col-span-6 md:col-start-7">
          Numinance is being built for the whole job: understand, plan, implement, verify, release, observe, recover, within the authority you configure.
        </p>
      </div>

      <div className="mt-10 md:mt-14">
        <SystemMachine />
      </div>

      <ol className="mt-8 grid border-t border-lp-line-strong md:mt-10 md:grid-cols-3">
        {NOTES.map((n) => (
          <li
            key={n.n}
            className="flex gap-4 border-b border-lp-line py-4 md:border-b-0 md:border-s md:px-5 md:py-5 md:first:border-s-0 md:first:ps-0"
          >
            <span
              aria-hidden
              className="font-pixel text-4xl leading-none text-lp-orange"
            >
              {n.n}
            </span>
            <div className="min-w-0">
              <h3 className="font-pixel text-[13px] leading-4 text-lp-ink">
                {n.k}
              </h3>
              <p className="mt-1.5 text-sm leading-relaxed text-lp-ink-2">
                {n.t}
              </p>
            </div>
          </li>
        ))}
      </ol>
      <div className="mt-10">
        <Ruler />
      </div>
    </Figure>
  );
}
