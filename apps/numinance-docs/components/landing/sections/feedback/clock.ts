/** A script clock: timers and Web Animations under `roots` that can be held together. */
export interface Clock {
  wait: (ms: number) => Promise<void>;
  hold: (held: boolean) => void;
  readonly held: boolean;
  stop: () => void;
}

interface Sleeper {
  left: number;
  since: number;
  timer: number;
  done: () => void;
}

export function createClock(roots: Element[]): Clock {
  let held = false;
  const sleepers = new Set<Sleeper>();
  const arm = (s: Sleeper) => {
    s.since = performance.now();
    s.timer = window.setTimeout(() => {
      sleepers.delete(s);
      s.done();
    }, Math.max(0, s.left));
  };
  return {
    wait(ms) {
      return new Promise((done) => {
        const s: Sleeper = { left: ms, since: 0, timer: 0, done };
        sleepers.add(s);
        if (!held) arm(s);
      });
    },
    hold(next) {
      if (next === held) return;
      held = next;
      for (const s of sleepers) {
        if (held) {
          clearTimeout(s.timer);
          s.left -= performance.now() - s.since;
        } else arm(s);
      }
      for (const root of roots)
        for (const a of root.getAnimations({ subtree: true })) {
          if (held && a.playState === 'running') a.pause();
          else if (!held && a.playState === 'paused') a.play();
        }
    },
    get held() {
      return held;
    },
    stop() {
      for (const s of sleepers) clearTimeout(s.timer);
      sleepers.clear();
    },
  };
}
