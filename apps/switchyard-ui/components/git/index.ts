/**
 * Git Components - GitOps "Humanity" Layer
 *
 * Components for injecting human context into deployment interfaces:
 * - AuthorAvatar: Smart avatar with GitHub/Gravatar/GeometricSeed fallbacks
 * - CommitLink: Clickable commit SHA linking to GitHub
 * - GeometricSeed: Deterministic SVG pattern avatar fallback
 * - InitialsAvatar: Simple text-based avatar fallback
 */

export { AuthorAvatar, CommitLink } from './AuthorAvatar';
export { GeometricSeed, InitialsAvatar } from './GeometricSeed';
