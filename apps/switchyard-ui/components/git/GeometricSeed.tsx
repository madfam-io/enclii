'use client';

import * as React from 'react';
import { cn } from '@/lib/utils';

// =============================================================================
// GEOMETRIC SEED - Deterministic Avatar Fallback
//
// Generates a unique geometric pattern based on a hash of the input string.
// In Enterprise mode: Neutral grayscale geometric shapes
// In Solarpunk mode: Bioluminescent glow with organic tints
// =============================================================================

interface GeometricSeedProps {
  /** String to hash (typically email or username) */
  seed: string;
  /** Size in pixels */
  size?: number;
  /** Additional class names */
  className?: string;
}

// Simple hash function for deterministic pattern generation
function hashString(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash; // Convert to 32-bit integer
  }
  return Math.abs(hash);
}

// Generate a seeded random number between 0 and 1
function seededRandom(seed: number, index: number): number {
  const x = Math.sin(seed * (index + 1)) * 10000;
  return x - Math.floor(x);
}

// Enterprise color palette (neutral grays)
const ENTERPRISE_COLORS = [
  '#94a3b8', // slate-400
  '#64748b', // slate-500
  '#475569', // slate-600
  '#334155', // slate-700
  '#1e293b', // slate-800
];

// Solarpunk color palette (organic bioluminescent)
const SOLARPUNK_COLORS = [
  '#00ff9d', // chlorophyll glow
  '#00d68f', // deep chlorophyll
  '#00b894', // moss
  '#00a884', // forest
  '#009874', // dark moss
];

type ShapeType = 'triangle' | 'square' | 'pentagon' | 'hexagon' | 'circle';

interface Shape {
  type: ShapeType;
  x: number;
  y: number;
  size: number;
  rotation: number;
  colorIndex: number;
  opacity: number;
}

function generateShapes(hash: number, count: number = 5): Shape[] {
  const shapes: Shape[] = [];
  const shapeTypes: ShapeType[] = ['triangle', 'square', 'pentagon', 'hexagon', 'circle'];

  for (let i = 0; i < count; i++) {
    shapes.push({
      type: shapeTypes[Math.floor(seededRandom(hash, i * 5) * shapeTypes.length)],
      x: seededRandom(hash, i * 5 + 1) * 80 + 10, // 10-90% of canvas
      y: seededRandom(hash, i * 5 + 2) * 80 + 10,
      size: seededRandom(hash, i * 5 + 3) * 30 + 15, // 15-45% size
      rotation: seededRandom(hash, i * 5 + 4) * 360,
      colorIndex: Math.floor(seededRandom(hash, i * 5 + 5) * 5),
      opacity: seededRandom(hash, i * 5 + 6) * 0.4 + 0.4, // 0.4-0.8
    });
  }

  return shapes;
}

function getPolygonPoints(type: ShapeType, cx: number, cy: number, size: number): string {
  const points: Array<[number, number]> = [];
  let sides: number;

  switch (type) {
    case 'triangle':
      sides = 3;
      break;
    case 'square':
      sides = 4;
      break;
    case 'pentagon':
      sides = 5;
      break;
    case 'hexagon':
      sides = 6;
      break;
    default:
      return ''; // Circle handled separately
  }

  for (let i = 0; i < sides; i++) {
    const angle = (i * 2 * Math.PI) / sides - Math.PI / 2;
    points.push([
      cx + size * Math.cos(angle),
      cy + size * Math.sin(angle),
    ]);
  }

  return points.map((p) => p.join(',')).join(' ');
}

export function GeometricSeed({ seed, size = 40, className }: GeometricSeedProps) {
  const hash = hashString(seed.toLowerCase());
  const shapes = generateShapes(hash, 4);

  // Background gradient based on hash
  const bgColorIndex = hash % 5;
  const bgColor1 = ENTERPRISE_COLORS[bgColorIndex];
  const bgColor2 = ENTERPRISE_COLORS[(bgColorIndex + 2) % 5];

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 100 100"
      className={cn('geometric-seed', className)}
      role="img"
      aria-label={`Avatar for ${seed}`}
    >
      <defs>
        {/* Enterprise gradient */}
        <linearGradient id={`bg-enterprise-${hash}`} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor={bgColor1} stopOpacity="0.3" />
          <stop offset="100%" stopColor={bgColor2} stopOpacity="0.5" />
        </linearGradient>

        {/* Solarpunk glow filter - activated via CSS */}
        <filter id={`glow-${hash}`} x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur stdDeviation="2" result="coloredBlur" />
          <feMerge>
            <feMergeNode in="coloredBlur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* Background circle */}
      <circle
        cx="50"
        cy="50"
        r="50"
        fill={`url(#bg-enterprise-${hash})`}
        className="geometric-seed-bg"
      />

      {/* Generated shapes */}
      {shapes.map((shape, index) => {
        const color = ENTERPRISE_COLORS[shape.colorIndex];
        const transform = `rotate(${shape.rotation}, ${shape.x}, ${shape.y})`;

        if (shape.type === 'circle') {
          return (
            <circle
              key={index}
              cx={shape.x}
              cy={shape.y}
              r={shape.size / 2}
              fill={color}
              fillOpacity={shape.opacity}
              stroke={color}
              strokeWidth="1"
              strokeOpacity={shape.opacity + 0.2}
              transform={transform}
              className="geometric-seed-shape"
            />
          );
        }

        return (
          <polygon
            key={index}
            points={getPolygonPoints(shape.type, shape.x, shape.y, shape.size / 2)}
            fill={color}
            fillOpacity={shape.opacity}
            stroke={color}
            strokeWidth="1"
            strokeOpacity={shape.opacity + 0.2}
            transform={transform}
            className="geometric-seed-shape"
          />
        );
      })}

      {/* Center accent shape */}
      <circle
        cx="50"
        cy="50"
        r="8"
        fill={ENTERPRISE_COLORS[hash % 5]}
        fillOpacity="0.8"
        className="geometric-seed-center"
      />
    </svg>
  );
}

// =============================================================================
// INITIALS FALLBACK - Simple text-based avatar
// =============================================================================

interface InitialsAvatarProps {
  name: string;
  size?: number;
  className?: string;
}

const INITIALS_COLORS = [
  'bg-blue-500',
  'bg-green-500',
  'bg-purple-500',
  'bg-orange-500',
  'bg-pink-500',
  'bg-teal-500',
  'bg-indigo-500',
  'bg-rose-500',
];

export function InitialsAvatar({ name, size = 40, className }: InitialsAvatarProps) {
  const initials = name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);

  const colorIndex = hashString(name) % INITIALS_COLORS.length;
  const bgColor = INITIALS_COLORS[colorIndex];

  return (
    <div
      className={cn(
        'inline-flex items-center justify-center rounded-full text-white font-medium',
        bgColor,
        className
      )}
      style={{ width: size, height: size, fontSize: size * 0.4 }}
      role="img"
      aria-label={`Avatar for ${name}`}
    >
      {initials}
    </div>
  );
}
