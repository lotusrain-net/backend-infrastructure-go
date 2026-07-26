# Theme System

The scaffold supports two first-class visual modes built on one token contract:

- Enterprise default: neutral surfaces, restrained accent use, dense-but-readable information presentation
- Cyberpunk alternate: dark chroma-rich surfaces, brighter accent ramps, stronger glow and gradient treatment

Theme switching must only swap tokens. Layout, semantics, accessibility behavior, and component APIs stay constant.

## Color Tokens

Base token groups:

- `bg.canvas`, `bg.surface`, `bg.surfaceRaised`, `bg.inverse`
- `fg.default`, `fg.muted`, `fg.inverse`
- `border.subtle`, `border.strong`
- `accent.primary`, `accent.secondary`, `accent.success`, `accent.warning`, `accent.danger`
- `focus.ring`
- `chart.1` through `chart.6`

Enterprise defaults:

- backgrounds lean light or graphite-neutral depending on mode needs
- accents are blue, teal, amber, and red with minimal saturation spill
- shadows are soft and short

Cyberpunk alternate:

- backgrounds are charcoal or midnight with cyan, magenta, and lime accent highlights
- borders may use faint luminous edge treatment, but content contrast must still satisfy WCAG 2.1 AA
- shadows can be longer and colored, but must not blur text or state cues

## Typography

- Primary UI font: `IBM Plex Sans`, falling back to `Segoe UI`, `sans-serif`
- Display or data-emphasis font: `Space Grotesk`, falling back to the primary stack
- Monospace font: `IBM Plex Mono`, falling back to `Consolas`, `monospace`

Rules:

- use the primary stack for navigation, forms, and tables
- use the display stack sparingly for dashboard headings, hero metrics, and mode-specific emphasis
- use the monospace stack for IDs, timestamps, cron expressions, and inline code-like values

## Components

- App shell: persistent sidebar on desktop, drawer on mobile, top-level skip link, and clear current-location indicator
- Cards and panels: reusable surface tokens with distinct resting, hover, selected, and loading states
- Data tables: compact defaults, sticky headers where practical, and explicit empty/error/loading treatments
- Forms: labels always visible, helper text adjacent to inputs, validation inline and programmatic
- Dialogs and drawers: trap focus, preserve escape semantics, and restore focus to the invoking control
- Charts and metric tiles: never rely on color alone; pair accent color with text, icon, or pattern support

## Interaction States

Every interactive component must expose:

- resting
- hover
- focus-visible
- active or pressed
- selected when applicable
- disabled
- loading
- error when applicable
- success or completed when applicable

State differences must be visible in both themes without depending solely on hue shifts.

## Responsive Behavior

- Mobile: single-column priority layout, drawer navigation, and stacked filters/actions
- Tablet: two-column dashboard rhythm with collapsible side regions
- Desktop: persistent navigation, denser table layouts, and multi-panel dashboards

Breakpoints may vary by implementation, but the scaffold must preserve:

- no horizontal scrolling for primary workflows at common viewport widths
- stable navigation access at every breakpoint
- readable tables through column prioritization, not text shrinkage
- touch-safe target sizing for mobile controls
