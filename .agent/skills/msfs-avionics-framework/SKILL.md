---
name: msfs-avionics-framework
description: >
  MSFS Avionics Framework (FSComponent) and EFB API skill for building Electronic Flight Bag
  applications in Microsoft Flight Simulator. Use when working on MSFS EFB apps, FSComponent-based
  UI, MapSystemBuilder maps, Subject-based data flow, or any TypeScript/JSX code targeting the
  @microsoft/msfs-sdk or @efb/efb-api packages. Covers: component lifecycle, Subject/EventBus
  reactive data, MapSystem with custom layers, EFB App registration, and critical antipatterns
  that cause silent failures in the EFB runtime environment.
---

# MSFS Avionics Framework

## Quick Orientation

FSComponent is a **React-like but NOT React** framework. JSX compiles via `FSComponent.buildComponent`. Key differences from React:

- **Class-based only** -- no functional components, no hooks
- **`render()` called exactly once** -- no re-renders on prop/state change
- **No virtual DOM** -- all dynamic updates through Subject subscriptions binding to DOM
- **`ref` is the primary pattern**, not an escape hatch

Official docs: https://microsoft.github.io/msfs-avionics-mirror/2024/docs/intro

## Antipatterns & Pitfalls

These bugs have been hit repeatedly. **Check every change against this list before committing.**

### AP-1: Treating render() as reactive (CRITICAL)

**Wrong:** Calling methods or reading values in `render()` expecting them to update.
```tsx
// BUG: this.getName() evaluated once at render, never updates
render() { return <div>{this.getName()}</div>; }
```
**Right:** Bind a Subject in `onAfterRender` and update DOM via `textContent`/`style`.
```tsx
private nameRef = FSComponent.createRef<HTMLSpanElement>();
render() { return <span ref={this.nameRef} />; }
onAfterRender() {
  this.nameSub = this.name.sub(v => { this.nameRef.instance.textContent = v; }, true);
}
```

### AP-2: Subscribing in the constructor (CRITICAL)

**Wrong:** Subject `.sub()` in constructor -- notifications are never delivered.
```tsx
constructor(props) {
  super(props);
  this.props.telemetry.sub(t => this.update(t)); // SILENT FAILURE
}
```
**Right:** Subscribe in `onAfterRender` (DisplayComponent) or `onAttached` (MapLayer).

### AP-3: Assuming SimVar/GPS/Clock publishers exist on the EFB bus (CRITICAL)

The EFB `EventBus` has **no ClockPublisher, no GNSSPublisher, no SimVar wiring**.

| What fails silently                              | Why                                    | Fix                                                                  |
|--------------------------------------------------|----------------------------------------|----------------------------------------------------------------------|
| `bus.getSubscriber<GNSSEvents>().on('gps-position')` | No GNSSPublisher                  | Drive position from HTTP telemetry Subject                           |
| `withOwnAirplanePropBindings(['position'], freq)`    | No SimVar publisher               | Pass empty `[]`, update module manually                              |
| `withClockUpdate(freq)`                              | No ClockPublisher, `realTime` dead | Add `setInterval(() => mapSystem.ref.instance.update(Date.now()), 1000)` |

### AP-4: Using raw project() coordinates as CSS positions

`MapProjection.project()` returns target-relative coordinates, not canvas-space.

**Wrong:** `style.left = projected[0] + 'px'` -- markers cluster at origin.

**Right:** Normalize via target offset:
```ts
const cx = projSize[0] / 2, cy = projSize[1] / 2;
projection.project(projection.getTarget(), targetVec);
projection.project(geoPoint, projVec);
const x = cx + (projVec[0] - targetVec[0]);
const y = cy + (projVec[1] - targetVec[1]);
```

### AP-5: Per-frame object allocation (GC pressure)

**Wrong:** `new GeoPoint()` and `Vec2Math.create()` inside `onUpdated`/`repositionMarkers`.

**Right:** Pre-allocate scratch objects as class fields:
```ts
private readonly targetVec = Vec2Math.create();
private readonly projVec = Vec2Math.create();
private readonly geoScratch = new GeoPoint(0, 0);
```
Then reuse with `.set()` each frame.

### AP-6: DOM thrashing via innerHTML reconstruction

**Wrong:** Rebuilding DOM tree with `innerHTML = '...'` on every data update.

**Right:** Build DOM once in `onAfterRender`/`buildOverlayDom`, cache element refs, update only `textContent` or `style` properties. This prevents layout thrash and GC pauses in Coherent GT.

### AP-7: SDK enum/type exports that don't exist at runtime

Some SDK types are **type-only exports** or unexported in certain versions:
- `EBingReference` -- not exported. Use numeric literal `0` (SEA).
- `MapOwnAirplanePropsKey` -- type-only. Use `'position' as MapOwnAirplanePropsKey`.
- `trackTrue` on OwnAirplanePropsModule is `Subject<number>`, not `NumberUnitSubject`. Use `.set(heading)` not `.set(heading, UnitType.DEGREE)`.

### AP-8: Resource leaks on destroy

Every subscription, interval, and event listener must be cleaned up:
```ts
private subscriptions: Subscription[] = [];
private intervalHandles: number[] = [];

onAfterRender() {
  this.subscriptions.push(this.props.data.sub(...));
  this.intervalHandles.push(window.setInterval(...));
  window.addEventListener('resize', this.resizeHandler);
}
destroy() {
  this.subscriptions.forEach(s => s.destroy());
  this.intervalHandles.forEach(h => window.clearInterval(h));
  window.removeEventListener('resize', this.resizeHandler);
  super.destroy();
}
```

### AP-9: Using `delete` on typed object properties

**Wrong:** `delete marker.beacon` -- violates TypeScript type shape.

**Right:** `marker.beacon = undefined` -- preserves object shape for V8 hidden classes.

## Component Lifecycle

### DisplayComponent (UI components)
```
constructor(props)  -->  render()  -->  onAfterRender(node)  -->  destroy()
                         [once]         [subscribe here]         [cleanup here]
```

### MapLayer (map layers)
```
constructor  -->  render()  -->  onAttached()  -->  onWake()
                   [once]        [subscribe]        [active]

onMapProjectionChanged(proj, flags)   -- projection updated
onUpdated(time, elapsed)              -- each map update cycle

onSleep()  -->  onDetached()  -->  destroy()
```

### EFB App
```
Efb.use(MyApp)  -->  install()  -->  render() returns AppView
                      [loadCss]

AppView lifecycle:
  onOpen()    -- first open
  onResume()  -- returning from background
  onPause()   -- going to background
  onClose()   -- destroying

  registerViews()  -- register pages/popups via appViewService
```

## API References

For detailed type signatures:
- **MSFS SDK API**: See [references/msfs-sdk-api.md](references/msfs-sdk-api.md)
- **EFB API**: See [references/efb-api.md](references/efb-api.md)

## MapSystemBuilder Pattern

```tsx
const builder = MapSystemBuilder.create(bus)
  .withProjectedSize(sizeSubject)
  .withClockUpdate(1)                    // NOTE: inert in EFB without ClockPublisher
  .withBing("bing")                      // adds MapTerrainColorsModule
  .withOwnAirplanePropBindings([], 1)    // empty -- manual drive in EFB
  .withModule("MyData", () => ({         // custom shared state module
    mySubject: someSubject,
  }))
  .withLayer("MyLayer", (ctx) =>
    <MyCustomLayer model={ctx.model} mapProjection={ctx.projection} />, 100)
  .build("css-class");

// REQUIRED: Initialize Bing terrain colors (otherwise renders black)
const terrain = mapSystem.context.model.getModule(MapSystemKeys.TerrainColors);
terrain.colors.set(buildEarthColors());  // 61-entry RGB-packed array
terrain.reference.set(0);                // 0 = sea level

// REQUIRED in EFB: Manual map update cycle
setInterval(() => mapSystem.ref.instance?.update(Date.now()), 1000);
```

## Subject Data Flow

```tsx
// Create with custom equality to prevent unnecessary subscriber cascades
const telemetry = Subject.create<TelData>(null, (a, b) =>
  a === b || (a && b && a.lat === b.lat && a.lon === b.lon));

// Subscribe (always in onAfterRender/onAttached, with initialNotify)
this.subs.push(telemetry.sub(value => { ... }, true));

// Derived values
const mapped = telemetry.map(t => t?.speed ?? 0);

// Pipe to another subject
source.pipe(target);                           // direct
source.pipe(target, (from, to) => transform);  // with transform

// Arrays
const items = ArraySubject.create<Item>([]);
items.set(newArray);     // replace all
items.insert(item, idx); // insert at index
items.removeAt(idx);     // remove by index
```

## Build & Toolchain

- Entry: `src/Phileas.tsx` bundled by Rollup to IIFE `dist/phileas.js`
- `@microsoft/msfs-sdk` is **external** (global `msfssdk` at runtime)
- JSX factory: `FSComponent.buildComponent`, fragment: `FSComponent.Fragment`
- SCSS auto-prefixed with `.efb-view.phileas` -- never add prefix manually
- Build constants: `BASE_URL`, `VERSION`, `BUILD_TIMESTAMP` (injected by Rollup)
- Assets referenced at runtime via `${BASE_URL}/assets/...`
