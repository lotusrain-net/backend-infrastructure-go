# Product Register

- Product: Backend Infrastructure Console
- Type: reusable internal operations console scaffold paired with the Go backend infrastructure in this repository
- Status: approved scaffold integration
- Frontend source of truth: sanitized import from `xuanli520/douyin_dashboard_frontend` at `main@3de3320`, documented in `docs/upstream/authorized-frontend.md`

## Users

- Platform engineers who need a ready-made admin shell for authentication, health, task scheduling, audit, and system operations flows
- Internal operators who manage users, roles, permissions, schedules, and runtime status
- Product teams that need a generic dashboard shell they can adapt without inheriting Douyin-specific business language or assets

## Purpose

The frontend exists to shorten time-to-first-product for teams adopting this backend infrastructure. It supplies a production-ready shell for login, navigation, administration, and operational dashboards while keeping the backend repository reusable and domain-neutral.

## Personality

- Enterprise default: calm, structured, and trustworthy for day-to-day operator work
- Cyberpunk alternate: high-contrast, neon-accented, and intentionally dramatic for demo, showcase, or dark-ops presentation modes

Both themes must share the same semantics, spacing logic, and interaction model so that switching themes never changes information architecture or task flow.

## Anti-References

The scaffold must not preserve or reintroduce:

- Douyin-branded copy, metrics, screenshots, mascots, or line-of-business terminology
- Product-specific pages for scraping rules, data-source collection, agent workbench, compass analysis, or creator-growth dashboards
- Business assets copied from the source project such as branded PNG sets or profile photography
- Any coupling to Python services, Playwright flows, or non-Go delivery assumptions

## Design Principles

- Reusable first: every retained screen pattern must make sense for a generic operations product
- Neutral language: labels, empty states, and help text must stay domain-agnostic
- Theme parity: enterprise and cyberpunk themes expose the same components and state cues
- Operational clarity: health, auth, and task outcomes should be immediately legible under load
- Accessibility over novelty: visual character is welcome, but interaction cost must stay low

## Accessibility

The integrated frontend must meet WCAG 2.1 AA for:

- color contrast in both supported themes
- keyboard navigation for all controls, overlays, and navigation landmarks
- visible focus indicators that remain distinct from hover and selected states
- form labels, error messaging, and status announcements for assistive technology
- responsive reading order and zoom support through 200% without loss of core workflow access
