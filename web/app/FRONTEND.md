# Frontend Development Guide

These rules apply to the Vite web app in `web/app`.

Chinese companion: `FRONTEND.zh.md`. This English document is the agent-facing source of truth.

## Scope

- Keep frontend source in `web/app/src`.
- Treat `web/static-dist` as generated output for Go embedding.
- Keep legacy comparison assets in `web/static` unless the task explicitly targets them.
- Prefer local conventions already present in `web/app` before adding new patterns or dependencies.

## Top-Level Structure

- `src/api/`: HTTP request wrappers and API boundary code.
- `src/bootstrap/`: app bootstrapping, providers, constants, and app-level wiring.
- `src/components/`: shared components used outside a single page.
- `src/models/`: pure data normalization, formatting, routing, and domain helpers.
- `src/pages/`: route-level pages and page-private modules.
- `src/shared/`: cross-cutting i18n, storage keys, realtime utilities, theme, styles, and generic helpers.

Do not create new top-level directories unless a module is clearly cross-cutting and does not fit the existing structure.

## Page Modules

- Put route-level screens under `src/pages/<PageName>/`.
- Put page-private components under `src/pages/<PageName>/components/`.
- Put page-private hooks, helpers, constants, and types next to the page that owns them.
- Move page-private code to `src/components`, `src/models`, or `src/shared` only after it has real cross-page reuse.
- Import page-private components through the page-local entrypoint, for example `./components`.

## Shared Components

- `src/components/ui/` contains presentation primitives and icons.
- `src/components/business/` contains shared app-aware components.
- Do not put page-private components in `src/components/`.
- Pure icon components belong under `src/components/ui/Icons/`.
- A component becomes business UI when it combines UI primitives with business state, labels, actions, or API-backed data.

## Component Naming

- Exported component packages use PascalCase directories.
- The primary implementation file uses the same PascalCase name as the component.
- Each exported component package has an `index.ts` entrypoint.
- `index.ts` files should only import required package CSS and re-export public symbols.
- Avoid kebab-case component directories such as `message-content` and vague grouping names such as `workspace-views`.

Example:

```text
src/pages/WorkspacePage/components/
  AgentDetailPane/
    AgentDetailPane.tsx
    AgentDetailPane.css
    index.ts
```

## Component Granularity

- Public components in `src/components/**` must be folder packages.
- Exported page components in `src/pages/<PageName>/components/**` should be folder packages.
- Tiny private subcomponents may stay inside the parent `.tsx` file when they are not exported and have no separate CSS, tests, types, or helpers.
- Promote a component to a folder package when it has CSS, tests, `types.ts`, `utils.ts`, constants, child components, two or more importers, or grows beyond about 150-200 lines.

## Imports

- Use the `@/` alias for shared app modules.
- Use relative imports for files inside the same feature or page package.
- Import shared components from package entrypoints, for example `@/components/ui` or `@/components/business/ProfileControls`.
- Do not add compatibility re-export folders for old paths after moving a component; update callers instead.
- Avoid importing page-private modules from another page. Promote truly shared code first.

## Styling

- Component-owned CSS lives next to the component and uses the component name, such as `AgentDetailPane.css`.
- Feature-level shared CSS can live in the feature components folder, such as `WorkspaceComponents.css`.
- Global styles and design tokens stay in `src/shared/styles/`.
- Prefer existing CSS variables and tokens before introducing new color, spacing, or shadow values.
- Keep CSS class names tied to component or feature semantics; avoid generic class names that can collide globally.
- Do not put page-specific styles in `src/shared/styles/`.

## State And Data

- Keep API request code in `src/api/`.
- Keep data shaping, normalization, formatting, and route parsing in `src/models/` when it is shared or large enough to test independently.
- Keep transient UI state inside the owning page or component.
- Add shared state only when state must be read or updated by multiple distant modules.
- Do not mix fetch calls, normalization, and rendering logic in one large component when the logic can be cleanly separated.

## i18n And Text

- Keep user-facing strings in the existing i18n message structure when the text is translated.
- Use translator functions passed into page-private components when that is the surrounding pattern.
- Do not hardcode new bilingual UI strings in components unless the existing code path is already untranslated.
- Keep internal code comments and developer docs in English unless a Chinese companion document is explicitly being maintained.

## Accessibility

- Use native buttons and form controls for interactive elements whenever possible.
- Icon-only buttons need `aria-label` and usually `title`.
- Preserve keyboard access for clickable non-button elements, or convert them to buttons.
- Keep visible focus states intact.
- Make loading, disabled, and error states explicit in the UI when actions can fail.

## Tests And Verification

- Run `pnpm --dir web/app typecheck` after TypeScript or import-path changes.
- Run `pnpm --dir web/app test` for the frontend Vitest suite.
- Run `pnpm --dir web/app exec vite build --outDir /private/tmp/csgclaw-web-build --emptyOutDir` when validating bundling without touching `web/static-dist`.
- Run `pnpm --dir web/app build` only when the generated `web/static-dist` output is intended.
- Add or update pure unit tests for data shaping, routing, formatting, parser, serializer, and state-transition helpers. Good targets include `src/models/**`, `src/shared/lib/**`, and logic-only helpers inside component packages.
- Put pure unit tests under `tests/models`, `tests/shared`, or a matching focused folder, and keep them small: table-test edge cases, invalid input, defaults, and regression cases without rendering React when a function call is enough.
- Use React Testing Library with jsdom for component behavior: render the public component, query by role/label/text, drive interactions with `userEvent.setup()`, and assert user-visible output, disabled/loading/error states, or emitted callbacks.
- Do not replace unit tests with component tests when logic is already extracted. Test pure helpers directly, then add one or two component tests for the wiring that a user can observe.
- Use browser or e2e verification only for behavior jsdom cannot represent well, such as layout, responsive behavior, canvas/media rendering, real browser APIs, or full app workflows.
- If a visual workflow changes, start the app or a dev server and verify the affected UI in a browser.

## Generated Output

- Do not manually edit `web/static-dist`.
- Use the app build pipeline to regenerate embedded assets.
- If a verification build writes to `web/static-dist` accidentally, call that out and clean it only with explicit approval or a clearly safe generated-output workflow.
