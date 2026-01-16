/**
 * Trellis Visualization - Topological Project View
 *
 * A hierarchical tree visualization showing:
 * Organization → Projects → Services
 *
 * Features:
 * - CSS Grid + SVG connectors (lightweight, no React Flow dependency)
 * - Expand/collapse project nodes
 * - CPU/RAM gauges at each service node
 * - Click-to-zoom navigation
 * - Theme-aware styling (Enterprise vs Solarpunk)
 */

export {
  TrellisView,
  type TrellisOrganization,
  type TrellisProject,
  type TrellisService,
} from './TrellisView';
