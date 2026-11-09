import { frameKey, plan, sample, finalFrame, type Frame } from './timeline';

type Sub = (f: Frame, dt: number) => void;

/**
 * The CLI figure's single animation clock. Exactly one rAF loop, alive only while a run still has something
 * to move, the figure is on screen and the tab is visible. `start()` renders the first frame synchronously
 * (so a click is visible at the very next paint), cancels whatever was running and replaces it; nothing
 * waits for a previous sequence. Subscribers write continuous values (typed text, positions, spin) straight
 * to the DOM; React is only told when a discrete value changes (`onKey`).
 */
export class CliClock {
  private p = plan(0);
  private t = 0;
  private last = 0;
  private raf = 0;
  private visible = false;
  private live = false;
  private subs = new Set<Sub>();
  frame: Frame = finalFrame(0);

  subscribe(fn: Sub) {
    this.subs.add(fn);
    return () => {
      this.subs.delete(fn);
    };
  }

  private emit(dt: number) {
    this.frame = sample(this.p, this.t);
    for (const s of this.subs) s(this.frame, dt);
  }

  private tick = (now: number) => {
    this.raf = 0;
    const dt = Math.min(64, now - this.last);
    this.last = now;
    this.t += dt;
    this.emit(dt);
    if (this.t < this.p.stop) this.kick();
  };

  private kick() {
    if (this.raf || !this.visible || !this.live) return;
    if (this.t >= this.p.stop) return;
    this.last = performance.now();
    this.raf = requestAnimationFrame(this.tick);
  }

  /** begin a run from zero, replacing any run in progress */
  start(wf: number) {
    cancelAnimationFrame(this.raf);
    this.raf = 0;
    this.p = plan(wf);
    this.t = 0;
    this.live = true;
    this.emit(0);
    this.kick();
  }

  /** jump to a run's settled end (reduced motion) */
  settle(wf: number) {
    cancelAnimationFrame(this.raf);
    this.raf = 0;
    this.live = false;
    this.p = plan(wf);
    this.t = this.p.stop + 1;
    this.emit(0);
  }

  setVisible(v: boolean) {
    this.visible = v;
    if (v) this.kick();
    else {
      cancelAnimationFrame(this.raf);
      this.raf = 0;
    }
  }

  dispose() {
    cancelAnimationFrame(this.raf);
    this.raf = 0;
    this.subs.clear();
  }
}

export { frameKey };
