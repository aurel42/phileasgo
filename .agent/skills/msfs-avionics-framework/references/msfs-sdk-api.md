# MSFS SDK API Reference (@microsoft/msfs-sdk)

## Table of Contents
- [FSComponent](#fscomponent)
- [DisplayComponent](#displaycomponent)
- [Subject](#subject)
- [ArraySubject](#arraysubject)
- [MappedSubject](#mappedsubject)
- [Subscribable / MutableSubscribable](#subscribable)
- [EventBus](#eventbus)
- [MapSystemBuilder](#mapsystembuilder)
- [MapLayer](#maplayer)
- [MapProjection](#mapprojection)
- [GeoPoint](#geopoint)
- [Vec2Math](#vec2math)
- [UnitType](#unittype)
- [VNode & ComponentProps](#vnode--componentprops)

---

## FSComponent

Static utility namespace -- the JSX factory.

```typescript
// JSX factory (tsconfig: jsxFactory = "FSComponent.buildComponent")
static buildComponent(type, props, ...children): VNode | null

// Create a ref to access DOM elements or component instances post-render
static createRef<T>(): NodeReference<T>

// Fragment factory (tsconfig: jsxFragmentFactory = "FSComponent.Fragment")
static Fragment(props: ComponentProps): DisplayChildren[] | undefined

// Imperatively render a VNode into a DOM element
static render(node: VNode, element: HTMLElement | SVGElement | null, position?: RenderPosition): void

// Remove a rendered element
static remove(element: HTMLElement | SVGElement | null): void

// Bind CSS classes from a SubscribableSet or record
static bindCssClassSet(...): Subscription | Subscription[]
```

## DisplayComponent

Abstract base class for all UI components.

```typescript
abstract class DisplayComponent<P extends ComponentProps, Contexts = []> {
  props: P & ComponentProps;

  // Called before render (rarely needed)
  onBeforeRender(): void;

  // Called exactly ONCE to produce the component's VNode tree
  abstract render(): VNode | null;

  // Called after the rendered DOM is attached -- subscribe to Subjects here
  onAfterRender(node: VNode): void;

  // Cleanup subscriptions, intervals, event listeners
  destroy(): void;
}
```

## Subject

Observable value container. Core reactive primitive.

```typescript
class Subject<T> {
  // Create with optional custom equality (prevents redundant notifications)
  static create<T>(
    value: T,
    equalityFunc?: (a: T, b: T) => boolean,
    mutateFunc?: (oldVal: T, newVal: T) => void
  ): Subject<T>;

  get(): T;
  set(value: T): void;
  notify(): void;  // Force notification even if value unchanged

  // Subscribe. initialNotify=true delivers current value immediately.
  sub(handler: (value: T) => void, initialNotify?: boolean, paused?: boolean): Subscription;

  // Derive a read-only mapped value
  map<M>(fn: (input: T, prev?: M) => M, equalityFunc?): MappedSubscribable<M>;

  // Pipe value changes into another MutableSubscribable
  pipe(to: MutableSubscribable<any, T>, paused?: boolean): Subscription;
  pipe<OI, OV>(to: MutableSubscribable<OV, OI>, map: (from: T, to: OV) => OI, paused?: boolean): Subscription;
}
```

## ArraySubject

Observable array with granular mutation methods.

```typescript
class ArraySubject<T> {
  static create<T>(arr?: T[]): ArraySubject<T>;

  get length(): number;
  set(arr: readonly T[]): void;        // Replace entire array
  insert(item: T, index?: number): void;
  insertRange(index: number | undefined, arr: readonly T[]): void;
  removeAt(index: number): void;
  removeItem(item: T): boolean;
  clear(): void;
}
```

## MappedSubject

Combines multiple Subscribables into a derived value.

```typescript
class MappedSubject<I extends any[], T> {
  // Auto-derive: output is tuple of all input values
  static create<I extends any[]>(...inputs): MappedSubject<I, Readonly<I>>;

  // Custom mapping function
  static create<I extends any[], T>(
    mapFunc: (inputs: Readonly<I>, prev?: T) => T,
    ...inputs
  ): MappedSubject<I, T>;

  pause(): this;
  resume(): this;
  destroy(): void;
}
```

## Subscribable

```typescript
interface Subscribable<T> {
  readonly isSubscribable: true;
  sub(handler: (value: T) => void, initialNotify?: boolean, paused?: boolean): Subscription;
  map<M>(fn, equalityFunc?): MappedSubscribable<M>;
  pipe(to, map?): Subscription;
}

interface MutableSubscribable<T, I = T> extends Subscribable<T> {
  readonly isMutableSubscribable: true;
  set(value: I): void;
  get(): T;
}

interface Subscription {
  readonly isAlive: boolean;
  readonly isPaused: boolean;
  pause(): this;
  resume(): this;
  destroy(): void;
}
```

## EventBus

Pub/sub message bus for decoupled communication.

```typescript
class EventBus {
  on(topic: string, handler: Handler<any>, paused?: boolean): Subscription;
  pub(topic: string, data: any, sync?: boolean, isCached?: boolean): void;
}
```

**EFB constraint:** The EFB bus lacks ClockPublisher, GNSSPublisher, and SimVar wiring. Events like `realTime` and `gps-position` never fire.

## MapSystemBuilder

Fluent builder for constructing map systems.

```typescript
class MapSystemBuilder {
  static create(bus: EventBus): MapSystemBuilder;

  // Projection setup
  withProjectedSize(size: ReadonlyFloat64Array | Subscribable<ReadonlyFloat64Array>): this;
  withTargetOffset(offset: ReadonlyFloat64Array): this;
  withRange(range: NumberUnitInterface<UnitFamily.Distance>): this;

  // Clock & updates (NOTE: inert in EFB without ClockPublisher)
  withClockUpdate(updateFreq: number | Subscribable<number>): this;

  // Map features
  withBing(bingId: string, options?, order?, cssClass?): this;  // Adds TerrainColors + Wxr modules
  withFollowAirplane(): this;
  withRotation(): this;
  withOwnAirplaneIcon(iconSize, iconFilePath, iconAnchor, cssClass?, order?): this;
  withOwnAirplanePropBindings(bindings: Iterable<...>, updateFreq?): this;

  // Custom modules, layers, controllers
  withModule(key: string, factory: (ctx) => any): this;
  withLayer(key: string, factory: (ctx) => VNode, order?: number): this;
  withController(key: string, factory: (ctx) => Controller): this;
  withContext(key: string, factory: (ctx) => any): this;

  // Lifecycle callbacks
  withInit(key: string, callback: (ctx) => void): this;
  withOnAfterRender(key: string, callback: (ctx) => void): this;
  withDestroy(key: string, callback: (ctx) => void): this;

  // Build returns CompiledMapSystem
  build(cssClass?: string): CompiledMapSystem;
}
```

**CompiledMapSystem** contains:
- `context.model` -- access modules via `getModule(key)`
- `context.projection` -- the MapProjection instance
- `ref.instance` -- the map component (call `.update(Date.now())` manually in EFB)
- `map` -- the VNode to render

## MapLayer

Abstract base for custom map layers.

```typescript
abstract class MapLayer<P extends MapLayerProps<any>> extends DisplayComponent<P> {
  // Lifecycle (in order)
  onAttached(): void;          // Subscribe to data modules here
  onWake(): void;
  onMapProjectionChanged(mapProjection: MapProjection, changeFlags: number): void;
  onUpdated(time: number, elapsed: number): void;  // Each map update cycle
  onSleep(): void;
  onDetached(): void;

  // Visibility
  isVisible(): boolean;
  setVisible(val: boolean): void;
  onVisibilityChanged(isVisible: boolean): void;

  // Access via props
  props.mapProjection: MapProjection;
  props.model: MapModel;  // getModule(key)
}
```

## MapProjection

```typescript
class MapProjection {
  getTarget(): GeoPointReadOnly;
  getRange(): number;                       // Great-arc radians
  getProjectedSize(): ReadonlyFloat64Array; // [width, height] in px

  // Project geo coords to screen coords (target-relative!)
  project(point: GeoPointInterface, out: Float64Array): Float64Array;

  // Queue projection changes (applied on next update cycle)
  setQueued(params: { target?, range?, scaleFactor? }): void;
}
```

**Critical:** `project()` returns coordinates relative to the map target, NOT canvas-space. Always normalize:
```ts
projection.project(projection.getTarget(), targetVec);
projection.project(point, projVec);
x = canvasCenterX + (projVec[0] - targetVec[0]);
y = canvasCenterY + (projVec[1] - targetVec[1]);
```

## GeoPoint

```typescript
class GeoPoint {
  constructor(lat: number, lon: number);
  readonly lat: number;
  readonly lon: number;

  set(lat: number, lon: number): this;
  set(other: LatLonInterface): this;
  distance(other: LatLonInterface): number;  // Great-arc radians
  bearingTo(other: LatLonInterface): number; // Degrees
  isValid(): boolean;
}
```

## Vec2Math

Static 2D vector math on Float64Array.

```typescript
class Vec2Math {
  static create(): Float64Array;              // [0, 0]
  static create(x: number, y: number): Float64Array;
  static set(x, y, vec): Float64Array;
  static add(v1, v2, out): Float64Array;
  static sub(v1, v2, out): Float64Array;
  static dot(v1, v2): number;
  static multScalar(v1, scalar, out): Float64Array;
  static abs(v1): number;                    // Magnitude
  static normalize(v1, out): Float64Array;
}
```

## UnitType

Common unit constants for conversions.

```typescript
// Distance
UnitType.METER, .FOOT, .MILE, .NMILE, .KILOMETER, .GA_RADIAN

// Angle
UnitType.DEGREE, .RADIAN, .ARC_MIN, .ARC_SEC

// Speed
UnitType.KNOT, .KPH, .MPH, .MPS, .FPM

// Duration
UnitType.SECOND, .MINUTE, .HOUR, .MILLISECOND

// Weight
UnitType.KILOGRAM, .POUND

// Pressure
UnitType.HPA, .IN_HG, .MB

// Temperature
UnitType.CELSIUS, .FAHRENHEIT, .KELVIN
```

## VNode & ComponentProps

```typescript
interface VNode {
  instance: HTMLElement | SVGElement | DisplayComponent<any> | string | number | null;
  props: any;
  children: VNode[] | null;
}

class ComponentProps {
  children?: DisplayChildren[];
  ref?: NodeReference<any>;
}

class NodeReference<T> {
  instance: T;  // Available after render
}

type DisplayChildren = VNode | string | number | Subscribable<any> | DisplayChildren[] | null;
```
