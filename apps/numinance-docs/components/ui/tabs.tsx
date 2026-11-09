'use client';

import { useRef } from 'react';
import { LayoutGroup, motion } from 'motion/react';
import { cn } from '@/lib/cn';
import { EASE, dur } from '@/lib/motion';

export type TabItem = { id: string; label: string; hint?: string };

/**
 * Accessible tablist (WAI-ARIA tabs pattern, arrow/Home/End keys) with a shared
 * animated selection indicator. Controlled via `value`/`onChange`; panels are
 * rendered by the caller with `tabPanelProps(id)`.
 */
export function Tabs({
  items,
  value,
  onChange,
  label,
  idBase,
  className,
  size = 'md',
}: {
  items: TabItem[];
  value: string;
  onChange: (id: string) => void;
  label: string;
  /** Unique id prefix (use React useId) shared with tabPanelProps. */
  idBase: string;
  className?: string;
  size?: 'sm' | 'md';
}) {
  const uid = idBase;
  const refs = useRef<Record<string, HTMLButtonElement | null>>({});
  const onKey = (e: React.KeyboardEvent) => {
    const i = items.findIndex((t) => t.id === value);
    let n = i;
    if (e.key === 'ArrowRight') n = (i + 1) % items.length;
    else if (e.key === 'ArrowLeft') n = (i - 1 + items.length) % items.length;
    else if (e.key === 'Home') n = 0;
    else if (e.key === 'End') n = items.length - 1;
    else return;
    e.preventDefault();
    onChange(items[n].id);
    refs.current[items[n].id]?.focus();
  };
  return (
    <LayoutGroup id={uid}>
      <div
        role="tablist"
        aria-label={label}
        onKeyDown={onKey}
        className={cn('relative inline-flex max-w-full overflow-x-auto rounded-full border border-nm-line bg-nm-raised p-1', className)}
      >
        {items.map((t) => {
          const active = t.id === value;
          return (
            <button
              key={t.id}
              ref={(el) => {
                refs.current[t.id] = el;
              }}
              role="tab"
              type="button"
              id={`${uid}-tab-${t.id}`}
              aria-selected={active}
              aria-controls={`${uid}-panel-${t.id}`}
              tabIndex={active ? 0 : -1}
              onClick={() => onChange(t.id)}
              className={cn(
                'relative z-10 whitespace-nowrap rounded-full font-medium transition-colors',
                size === 'md' ? 'min-h-10 px-5 text-sm' : 'min-h-9 px-3.5 text-[0.8rem]',
                active ? 'text-nm-bg' : 'text-nm-muted hover:text-nm-fg',
              )}
            >
              {active && (
                <motion.span
                  layoutId="tab-pill"
                  className="absolute inset-0 -z-10 rounded-full bg-nm-fg"
                  transition={{ duration: dur.tab, ease: EASE }}
                />
              )}
              {t.label}
            </button>
          );
        })}
      </div>
    </LayoutGroup>
  );
}

/** ARIA props for the panel that belongs to tab `id`; pass the same `idBase` given to <Tabs>. */
export function tabPanelProps(idBase: string, id: string) {
  return { id: `${idBase}-panel-${id}`, role: 'tabpanel' as const, 'aria-labelledby': `${idBase}-tab-${id}`, tabIndex: 0 };
}
