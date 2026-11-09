import { cn } from '@/lib/cn';
import { Reveal } from '@/components/motion/reveal';

/** Section frame: consistent vertical rhythm, hairline top rule, eyebrow + heading block. */
export function Section({
  id,
  eyebrow,
  title,
  lede,
  children,
  className,
  bleed = false,
}: {
  id?: string;
  eyebrow?: string;
  title?: React.ReactNode;
  lede?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
  bleed?: boolean;
}) {
  return (
    <section id={id} className={cn('relative border-t border-nm-line py-24 md:py-32', className)}>
      <div className="nm-container">
        {(eyebrow || title || lede) && (
          <Reveal className="mb-12 max-w-3xl md:mb-16">
            {eyebrow && <p className="nm-eyebrow mb-5">{eyebrow}</p>}
            {title && <h2 className="nm-h2">{title}</h2>}
            {lede && <p className="nm-lede mt-6">{lede}</p>}
          </Reveal>
        )}
        {bleed ? null : children}
      </div>
      {bleed ? children : null}
    </section>
  );
}
