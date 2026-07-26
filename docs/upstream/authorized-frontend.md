# Authorized Frontend Upstream

- Upstream repository: `xuanli520/douyin_dashboard_frontend`
- Authorized branch and revision: `main@3de3320`
- Import policy: selective reuse only; generic infrastructure patterns may be adapted, while Douyin business flows and branded assets are excluded

## Approved Source Categories

The scaffold may retain and genericize patterns from these categories:

- authentication shell and login flow structure
- app shell layout, navigation, and responsive sidebar or drawer behavior
- generic admin pages for users, roles, permissions, login audit, profile, and system settings
- reusable UI primitives, dialog, drawer, dropdown, form, table, tabs, tooltip, toast, and empty-state components
- generic dashboard widgets, metric cards, charts, and operational status framing
- task scheduling, queue visibility, and other reusable operations-console workflows
- test harnesses, linting, typechecking, and local health endpoint conventions that support the frontend runtime

## Explicit Exclusions

Do not import or preserve:

- route groups or screens tied to Douyin business operations, including `agent-workbench`, `compass`, `data-center`, `data-source`, and `scraping-rule`
- Douyin-specific copy, KPI names, growth language, creator analytics framing, or collection-job business logic
- branded or business-specific imagery under `src/assets`, including the numbered PNG set and profile photo
- product-specific service modules that assume Douyin APIs or business entities

## Adaptation Rules

- Rename retained flows, copy, and IA to generic operations-console language before exposing them in this repository
- Keep the backend contract independent of any upstream frontend naming
- Preserve WCAG 2.1 AA requirements and the dual-theme token contract from `DESIGN.md`
