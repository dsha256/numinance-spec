import type { ModuleId } from './model';

/** 8x8 pixel icons for the plates, one per module (filled even-odd) */
export const ICON: Record<ModuleId, string> = {
  engine: 'M1 1h6v4H1zM3 6h2v1H3zM2 7h4v1H2z',
  kernel: 'M3 0h2v2H3zM1 2h6v6H1zM3 4h2v2H3z',
  config: 'M1 1h6v1H1zM1 3h6v1H1zM1 5h6v1H1zM1 7h4v1H1z',
  adapters: 'M2 0h1v2H2zM5 0h1v2H5zM1 2h6v3H1zM3 5h2v3H3z',
  evidence: 'M0 2h3v4H0zM5 2h3v4H5zM3 3h2v2H3z',
  boundary: 'M0 3h8v1H0zM0 6h8v1H0zM1 1h1v7H1zM6 1h1v7H6z',
};
