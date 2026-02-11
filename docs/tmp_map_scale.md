# Implementation Plan: The Navigator’s Dual-Scale Bar

## I. Architectural Foundation
* **Component Structure:** Create a component for the Scale Bar.
* **State The scale must only recalculate on snap-zoom events.

## II. The Mathematical Translation
* **Coordinate Reference:** Use the map’s center latitude to calculate the ground distance. Remember that on a Mercator projection, the physical distance represented by a pixel changes as you move from the equator toward the poles.
* **Unit Conversion:** * Establish a base measurement in meters.
    * Convert meters to Kilometers ($m / 1000$).
    * Convert meters to Nautical Miles ($m / 1852$).
* **Rounding Logic:** Implement an algorithm to find "clean" numbers (e.g., 10, 50, 100) for the scale divisions so the bar doesn't display awkward decimals like 13.47 knots.

## III. Visual Cartography (The "Verne" Aesthetic)
* **The Dual-Axis Layout:** Construct the scale with two parallel horizontal lines. The top axis shall represent the terrestrial metric system (Kilometers), and the bottom axis shall represent the maritime standard (Nautical Miles).
* **Ornate Labeling:** Place the unit labels at the far right of each axis. Use the established role-label font without size overrides.
* **Segmented Division:** Instead of a solid line, use a "checkered" or "alternating" bar style (common in antique maps) to indicate distance increments.
* **The Compass Anchor:** Position the entire assembly in the bottom-left corner. Do NOT enter in the r-tree, do not check collsions. Put it visually UNDER other map elements.
Since it doesn't collide, and is probably quite filigrane, it doesn't need to be small. Let's aim for a fourth to a third of the map's width. 
For the numbers, use the existing role-numbers-small (I think that's the name, you'll find it, it's what we use to render the API stats in the dashboard)
