# Artistic Map Symbol Styles

This document outlines the available decorative styles and parameters for POI markers and symbols in the PhileasGo Artistic Map.

## Core Parameters

When registering a `LabelCandidate` in the `PlacementEngine`, custom styling can be passed via the `custom` property.

```typescript
{
  id: string,
  // ... coordinates ...
  type: 'poi',
  custom: {
    halo?: 'normal' | 'organic' | 'neon-cyan' | 'neon-pink' | 'selected' | 'next';
    silhouette?: boolean;
    weight?: number; // Stroke weight in pixels
    color?: string;  // Stroke color (CSS)
  }
}
```

## Halo Effects (`halo`)

Halos are rendered using `drop-shadow` filters for organic, shape-hugging glows.

| Style | Description | Color Key / Hex |
| :--- | :--- | :--- |
| `normal` | Default paper cutout effect. | `normalHalo` (#f4ecd8) |
| `organic` | Soft, irregular ink smudge. | `organicSmudge` (#3a2a1d) |
| `neon-cyan` | Cyberpunk-style cyan glow. | `neonCyan` (#00f3ff) |
| `neon-pink` | Cyberpunk-style pink glow. | `neonPink` (#ff00ff) |
| `selected` | Active narration glow. | `selectedHalo` (#264653) |
| `next` | Upcoming narration glow. | `nextHalo` (#F4A261) |

## Silhouette (`silhouette`)

- **Effect**: Converts the icon into a flat, solid-colored shape.
- **Logic**: `contrast(0) brightness(0)`
- **Behavior**: Removes all internal details while retaining the overall contour. Usually paired with a `normal` halo for contrast.

## Outline Styles

Outlines are applied directly to the SVG paths within the icon.

- **`weight`**: Controls the `stroke-width`. Standard weight is `0.8px`. "Heavy Ink" is typically `1.8px`.
- **`color`**: Controls the `stroke` color. Defaults to charcoal black (`#0a0805`).

## Special Elements

### 1. Compass Rose (`type: 'compass'`)
- **Priority**: 200 (Highest).
- **Behavior**: Land-locked persistence. Seeks viewport corners based on aircraft heading.
- **Style**: Jules Verne-inspired hand-drawn SVG, 50% opacity.

### 2. Red Wax Seal
- **Placement**: Automatically rendered behind active (`currentNarrated` or `preparing`) POIs.
- **Z-Index**: 14 (explicitly below the icon at 15).
- **Aesthetic**: Physical, opaque red wax with glossy specular highlights and irregular edges.
