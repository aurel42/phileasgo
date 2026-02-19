# EFB API Reference (@efb/efb-api)

Asobo Studio's Electronic Flight Bag framework layered on top of @microsoft/msfs-sdk.
Type declarations at: `PackageSources/efb_api/dist/*.d.ts`

## Table of Contents
- [App Registration](#app-registration)
- [App Class](#app-class)
- [AppView](#appview)
- [AppViewService](#appviewservice)
- [GamepadUiView](#gamepaduiview)
- [UI Components](#ui-components)
- [Notifications](#notifications)
- [Settings](#settings)

---

## App Registration

```typescript
import { Efb } from "@efb/efb-api";

// Register an app with the EFB container
Efb.use(MyApp);

// Load CSS (call in install())
Efb.loadCss(uri: string): Promise<void>;

// Load external JS
Efb.loadJs(uri: string): Promise<void>;
```

## App Class

```typescript
abstract class App<T extends AppOptions = AppOptions> {
  // REQUIRED overrides
  abstract get name(): string;      // Display name in EFB menu
  abstract get icon(): string;      // Icon URL (use ${BASE_URL}/assets/...)
  abstract render(): TVNode<AppView>;

  // Boot/suspend behavior
  BootMode: AppBootMode;      // COLD=0, WARM=1, HOT=2
  SuspendMode: AppSuspendMode; // SLEEP=0, TERMINATE=1

  // Optional
  get internalName(): string;  // Default: constructor.name
  install(props: AppInstallProps): Promise<void>;  // Load CSS, init resources
  get bus(): EventBus;
}

enum AppBootMode {
  COLD = 0,      // No JSX until launched
  WARM = 1,      // JSX created but not mounted
  HOT = 2        // JSX created and mounted immediately
}

enum AppSuspendMode {
  SLEEP = 0,     // Paused when user navigates away
  TERMINATE = 1  // Destroyed when user navigates away
}
```

**Typical pattern:**
```tsx
class MyApp extends App {
  get name() { return "My App"; }
  get icon() { return `${BASE_URL}/assets/app-icon.svg`; }
  BootMode = AppBootMode.HOT;
  SuspendMode = AppSuspendMode.SLEEP;

  async install(_props: AppInstallProps) {
    Efb.loadCss(`${BASE_URL}/myapp.css`);
  }

  render(): TVNode<MyAppView> {
    return <MyAppView bus={this.bus} />;
  }
}
Efb.use(MyApp);
```

## AppView

Owns the app's view stack and lifecycle.

```typescript
abstract class AppView<T extends AppViewProps = AppViewProps> extends DisplayComponent<T> {
  protected defaultView?: string;

  // Lifecycle
  onOpen(): void;     // First time opened
  onClose(): void;    // Being destroyed
  onResume(): void;   // Returning from background
  onPause(): void;    // Going to background

  // Register sub-views
  protected registerViews(): void;

  // Access
  protected get appViewService(): AppViewService;
  protected get bus(): EventBus;

  render(): TVNode<NodeInstance, T>;
  destroy(): void;
}

interface AppViewProps extends ComponentProps {
  appViewService?: AppViewService;
  bus?: EventBus;
}
```

## AppViewService

Manages the view stack (pages and popups).

```typescript
class AppViewService {
  // Register views
  registerPage(key: string, factory: () => TVNode, options?): UiViewEntry;
  registerPopup(key: string, factory: () => TVNode, options?): UiViewEntry;

  // Navigation
  open<Ref>(key: string): PublicUiViewEntry<Ref>;
  goBack(steps?: number): PublicUiViewEntry | undefined;
  isActive(key: string): boolean;

  // Current view observable
  readonly currentUiView: Subscribable<UiViewEntry | null>;
}

enum ViewBootMode {
  COLD = 0,  // JSX created on demand
  HOT = 1    // JSX mounted immediately
}

enum ViewSuspendMode {
  SLEEP = 0,      // Kept alive when navigating away
  TERMINATE = 1   // Destroyed when navigating away
}
```

## GamepadUiView

Base class for views with gamepad/controller support.

```typescript
abstract class GamepadUiView<T extends HTMLElement, P extends UiViewProps = UiViewProps>
  extends UiView<P> {

  abstract readonly tabName: string | Subscribable<string>;
  protected readonly gamepadUiViewRef: NodeReference<T>;
}

// UiViewProps
interface UiViewProps extends ComponentProps {
  appViewService?: AppViewService;
  bus?: EventBus;
}
```

## UI Components

### TTButton (Text-Translated Button)
```typescript
class TTButton extends DisplayComponent<TTButtonProps> {
  // TTButtonProps = ButtonProps & TTProps & GamepadUiComponentProps
}

interface ButtonProps extends GamepadUiComponentProps {
  callback?: (e: MouseEvent) => void;
  hoverable?: MaybeSubscribable<boolean>;
  selected?: MaybeSubscribable<boolean>;
}
```

Usage: `<TTButton key="Label" callback={() => doSomething()} />`

### List
```typescript
class List<T> extends GamepadUiComponent<HTMLDivElement, ListProps<T>> {
  scrollToItem(index: number): void;
}

interface ListProps<T> extends GamepadUiComponentProps {
  data: SubscribableArray<T>;           // Must be ArraySubject
  renderItem: (item: T, index: number) => VNode | null;
  refreshOnUpdate?: boolean;            // Re-render items when data changes
  isScrollable?: boolean;
  isListVisible?: Subscribable<boolean>;
}
```

Usage:
```tsx
<List
  data={this.uiItems}
  renderItem={(item) => <div>{item.name}</div>}
  refreshOnUpdate={true}
/>
```

### Other Components
- `Button` -- base button with callback, hoverable, selected state
- `TT` -- text translation component (`<TT key="translation.key" />`)
- `Accordion` -- collapsible sections
- `TextInput` / `TextArea` -- text entry
- `Slider` -- range input
- `Switch` -- toggle
- `Progress` / `ProgressBar` / `CircularProgress` -- progress indicators
- `Tag` -- chip/badge display
- `Tooltip` -- hover tooltip

### Common Props Pattern
```typescript
interface GamepadUiComponentProps extends ComponentProps {
  appViewService?: AppViewService;
  class?: ClassProp;
  style?: StyleProp;
  disabled?: boolean | Subscribable<boolean>;
  visible?: boolean | Subscribable<boolean>;
}

type MaybeSubscribable<T> = T | Subscribable<T>;
```

## Notifications

```typescript
class NotificationManager {
  static getManager(bus: EventBus): NotificationManager;
  addNotification(notif: EfbNotification): void;
  clearNotifications(): void;
  readonly unseenNotificationsCount: Subscribable<number>;
}

interface EfbNotification {
  uuid: string;
  type: 'temporary' | 'permanent';
  style: 'info' | 'warning' | 'error' | 'success';
  description: string;
  delayMs: number;
  icon?: string | VNode;
}
```

## Settings

Access EFB-wide settings via managers on the App class:

```typescript
// Unit preferences
app.unitsSettingsManager.navAngleUnits  // Subscribable<NavAngleUnit>

// EFB display settings
app.efbSettingsManager  // mode (2D/3D), size, orientation, brightness
```

Unit setting modes:
- Speed: `KTS` | `KPH`
- Distance: `NM` | `KM`
- Altitude: `FT` | `M`
- Temperature: `F` | `C`
- Time: `local-12` | `local-24`
