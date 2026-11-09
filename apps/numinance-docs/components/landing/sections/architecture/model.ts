/**
 * What the architecture scene says, as data: six modules, what each may do (swap or inspect only), the
 * readout the monitor prints for every option, and what a cable's state means. The engine, the panel and
 * the static fallback all read this one table, so no control can exist without an effect.
 *
 * Sources: the Numinance technical design (fixed trust kernel, customer-owned configuration, replaceable
 * typed adapters, identity separation, execution boundary) and the public spec tree. Pre-alpha: nothing
 * here claims availability. The boundary semantics are folded in from the former customer-boundary diagram.
 */

export type ModuleId = 'engine' | 'kernel' | 'config' | 'adapters' | 'evidence' | 'boundary';

/** top to bottom: the order of the panel; the stack is built from the bottom */
export const ORDER: readonly ModuleId[] = ['engine', 'kernel', 'config', 'adapters', 'evidence', 'boundary'];

/** how a cable's data runs: in from the customer's side, out to it, both ways, or a decision with no flow */
export type Flow = 'in' | 'out' | 'both' | 'none';
/** solid: shipping design; dashed: designed, not built */
export type CableLook = 'solid' | 'dashed';

export interface Readout {
  title: string;
  /** at most six lines of at most 40 characters */
  lines: readonly string[];
  /** what the cable says in this state: shown to assistive tech and in the status bar */
  cable?: CableLook;
}

export interface Option {
  title: string;
  /** a chip or plate too narrow for the title */
  short: string;
  readout: Readout;
}

export interface Group {
  id: string;
  /** a line label for a module with several ports */
  label?: string;
  options: readonly Option[];
}

export interface ModuleDef {
  id: ModuleId;
  /** the plate's role line, and the panel's name */
  name: string;
  role: string;
  /** swappable: the arm fetches a cartridge. fixed: the arm may only touch it, to read it out. */
  swap: boolean;
  /** shown next to a fixed row */
  tag: string;
  cable: { flow: Flow; label: string };
  groups: readonly Group[];
  /** the plate's name line for a selection; defaults to the option's short name */
  plate?: (sel: readonly number[], last: number) => string;
}

const L = (title: string, ...lines: string[]) => ({ title, lines });

export const MODULES: Record<ModuleId, ModuleDef> = {
  engine: {
    id: 'engine',
    name: 'Engine',
    role: 'Proposes bounded work',
    swap: false,
    tag: 'private',
    cable: { flow: 'out', label: 'proposals leave as typed requests' },
    groups: [
      {
        id: 'engine',
        options: [
          {
            title: 'Proposes',
            short: 'Proposes',
            readout: L('ENGINE / private', 'reasons over an objective', 'and the context it may read', 'proposes bounded work units', 'proprietary; only its boundary', 'is public'),
          },
          {
            title: 'Cannot authorize',
            short: 'No authority',
            readout: L('ENGINE / no authority', 'a proposal is not an authorization', 'policy is decided outside the model', 'it cannot grant itself rights', 'it cannot approve its own work', 'intelligence is not authority'),
          },
        ],
      },
    ],
    plate: () => 'Private engine',
  },
  kernel: {
    id: 'kernel',
    name: 'Trust kernel',
    role: 'Fixed. Inspect only',
    swap: false,
    tag: 'fixed',
    cable: { flow: 'none', label: 'decisions, not data' },
    groups: [
      {
        id: 'kernel',
        options: [
          {
            title: 'Policy decision',
            short: 'Policy',
            readout: L('KERNEL / policy decision point', 'decides before a privileged act', 'issues scoped grants: paths, tools,', 'budget, environment, residency, expiry', 'the model never grants itself rights', 'fixed: not swappable'),
          },
          {
            title: 'Credential broker',
            short: 'Broker',
            readout: L('KERNEL / credential broker', 'workers get handles, not secrets', 'an egress proxy injects the token', 'egress is denied by default', 'design intent. pre-alpha', 'fixed: not swappable'),
          },
          {
            title: 'Plugin host',
            short: 'Host',
            readout: L('KERNEL / plugin host + sandbox', 'adapters run behind typed ports', 'every call is checked against a grant', 'model workers stay in a sandbox', 'design intent. pre-alpha', 'fixed: not swappable'),
          },
          {
            title: 'Evidence writer',
            short: 'Writer',
            readout: L('KERNEL / evidence writer', 'records are written by the runner', 'signed and hash-linked', 'model text cannot certify its work', 'fixed: not swappable'),
          },
          {
            title: 'Circuit breaker',
            short: 'Breaker',
            readout: L('KERNEL / circuit breaker', 'a customer-local stop', 'halts privileged actions', 'lives inside your boundary', 'fixed: not swappable'),
          },
        ],
      },
    ],
    plate: () => 'Fixed kernel',
  },
  config: {
    id: 'config',
    name: 'Configuration',
    role: 'Yours to edit',
    swap: true,
    tag: 'yours',
    cable: { flow: 'in', label: 'your configuration flows in' },
    groups: [
      {
        id: 'config',
        options: [
          {
            title: 'SDLCProfile',
            short: 'SDLC',
            readout: L('CONFIG / SDLCProfile', 'your delivery lifecycle', 'its stages, gates and checks', 'versioned in .numinance/ with your code', 'customer-owned'),
          },
          {
            title: 'Playbook',
            short: 'Playbook',
            readout: L('CONFIG / Playbook', 'how a class of work is done', 'steps and checks per task type', 'qualified before autonomy rises', 'customer-owned'),
          },
          {
            title: 'AutonomyPolicy',
            short: 'Autonomy',
            readout: L('CONFIG / AutonomyPolicy', 'OBSERVE > SUGGEST > IMPLEMENT > PR', '> MERGE > STAGE > RELEASE > OPERATE', 'a ceiling per scope, never self-raised', 'raised only by human acceptance', 'customer-owned'),
          },
          {
            title: 'ControlPack',
            short: 'Controls',
            readout: L('CONFIG / ControlPack', 'controls your organization requires', 'checks that must pass', 'a floor that configuration cannot lower', 'customer-owned'),
          },
          {
            title: 'BudgetPolicy',
            short: 'Budget',
            readout: L('CONFIG / BudgetPolicy', 'caps on spend and attempts', 'bounded repair, then stop', 'design default: two repair attempts', 'customer-owned'),
          },
          {
            title: 'ResidencyPolicy',
            short: 'Residency',
            readout: L('CONFIG / ResidencyPolicy', 'where code and data may go', 'enforced at the egress boundary', 'constrains runners and executors', 'customer-owned'),
          },
        ],
      },
    ],
    plate: (sel) => MODULES.config.groups[0].options[sel[0]].title,
  },
  adapters: {
    id: 'adapters',
    name: 'Adapters',
    role: 'Replaceable, behind typed ports',
    swap: true,
    tag: 'swap',
    cable: { flow: 'both', label: 'typed requests and results' },
    groups: [
      {
        id: 'executor',
        label: 'Executor',
        options: [
          {
            title: 'Claude',
            short: 'Claude',
            readout: L('ADAPTER / executor: Claude', 'port: authorexecutor', 'a worker, never the authority', 'its credentials are brokered', 'the kernel and evidence stay as they are'),
          },
          {
            title: 'Codex',
            short: 'Codex',
            readout: L('ADAPTER / executor: Codex', 'port: authorexecutor', 'a worker, never the authority', 'swapped without touching the kernel', 'the same policy decides, the same', 'evidence is written'),
          },
          {
            title: 'Another harness',
            short: 'Other',
            readout: L('ADAPTER / executor: your harness', 'port: authorexecutor', 'vendor-neutral capabilities,', 'declared by the adapter', 'a future agent plugs in here', 'and gets no more authority'),
          },
        ],
      },
      {
        id: 'delivery',
        label: 'Source + CI',
        options: [
          {
            title: 'GitHub + Actions',
            short: 'GitHub',
            readout: L('ADAPTER / source + CI: GitHub', 'ports: scm.v1, ciexecutor.v1', 'official providers in the public tree', 'presence in the tree is not a', 'readiness claim. pre-alpha'),
          },
          {
            title: 'Your provider',
            short: 'Yours',
            readout: L('ADAPTER / source + CI: yours', 'ports: scm.v1, ciexecutor.v1', 'implement the typed port, keep your', 'existing pipelines where they run', 'calls pass the plugin host'),
          },
        ],
      },
      {
        id: 'context',
        label: 'Context',
        options: [
          {
            title: 'Repository docs',
            short: 'Repo docs',
            readout: L('ADAPTER / context: repository', 'port: knowledge.v1', 'authorized, cited, bounded context', 'lower trust: it cannot change', 'authority rules'),
          },
          {
            title: 'Connected sources',
            short: 'Connected',
            readout: L('ADAPTER / context: connected', 'port: knowledge.v1', 'issues and docs you authorize', 'cited, redacted, stage-bounded', 'no copied knowledge base in V1'),
          },
        ],
      },
    ],
    plate: (sel, last) => {
      const g = MODULES.adapters.groups[last];
      return `${g.label}: ${g.options[sel[last]].short}`;
    },
  },
  evidence: {
    id: 'evidence',
    name: 'Evidence',
    role: 'Verified outside the runtime',
    swap: false,
    tag: 'fixed',
    cable: { flow: 'out', label: 'records and digests flow out' },
    groups: [
      {
        id: 'evidence',
        options: [
          {
            title: 'Signed records',
            short: 'Records',
            readout: L('EVIDENCE / records', 'signed, hash-linked records', 'written by the runner, not the model', 'exported as evidence/v1', 'a public format'),
          },
          {
            title: 'Trust roots',
            short: 'Trust roots',
            readout: L('EVIDENCE / trust roots', 'the verifier needs trust material', 'you pin it. it is not carried', 'inside the export', 'unauthenticated, self-consistent,', 'verified: three different claims'),
          },
          {
            title: 'Offline check',
            short: 'Offline',
            readout: L('EVIDENCE / offline verification', 'verify apart from the engine', 'no live system required', 'reconciled with external systems', 'accountability, not proof of', 'code correctness'),
          },
        ],
      },
    ],
    plate: () => 'Verify offline',
  },
  boundary: {
    id: 'boundary',
    name: 'Execution boundary',
    role: 'Where work runs',
    swap: true,
    tag: 'yours',
    cable: { flow: 'both', label: 'refs and digests cross, not source' },
    groups: [
      {
        id: 'boundary',
        options: [
          {
            title: 'Managed',
            short: 'Managed',
            readout: L('BOUNDARY / managed', 'vendor control plane, your runtime', 'work and credentials stay in your', 'runtime. in: intent, plan, decision', 'out: status, evidence digests', 'never: source, prompts, secrets'),
          },
          {
            title: 'Sovereign',
            short: 'Sovereign',
            readout: L('BOUNDARY / sovereign', 'you operate control plane and runtime', 'policy, execution and evidence', 'all sit behind your boundary', 'nothing is drawn into it from outside'),
          },
          {
            title: 'Air-gapped',
            short: 'Air-gapped',
            readout: { title: 'BOUNDARY / air-gapped', lines: ['designed, not built in year one', 'no network path to a vendor plane', 'shown dashed: this is not shipping', 'pre-alpha design'], cable: 'dashed' },
          },
        ],
      },
    ],
    plate: (sel) => MODULES.boundary.groups[0].options[sel[0]].title,
  },
};

/** one selection per group, per module */
export type Selection = Record<ModuleId, number[]>;

export function defaultSelection(): Selection {
  const out = {} as Selection;
  for (const id of ORDER) out[id] = MODULES[id].groups.map(() => 0);
  return out;
}

export interface Pick {
  mod: ModuleId;
  group: number;
  option: number;
}

export function readoutOf({ mod, group, option }: Pick): Readout {
  return MODULES[mod].groups[group].options[option].readout;
}

export function cableOf(sel: Selection, mod: ModuleId): CableLook {
  return MODULES[mod].groups.reduce<CableLook>((look, g, i) => g.options[sel[mod][i]].readout.cable ?? look, 'solid');
}

/** the disclosure the figure always carries */
export const DISCLOSURE = 'Illustrative architecture / pre-alpha design';
