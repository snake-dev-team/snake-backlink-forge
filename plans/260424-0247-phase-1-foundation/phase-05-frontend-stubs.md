---
name: Frontend stubs (landing + extension + shared-types)
phase: 05
status: pending
priority: P1
estimated_effort: 1.5h
blockedBy: [phase-01-monorepo-backbone]
blocks: [phase-06-quality-gates]
agents: [fullstack-developer]
---

# Phase 05 — Frontend Stubs

## Context Links

- MASTER_PROMPT §1.2 ADR (Next.js 15, Svelte 5, Vite 5, TS 5.5+ locked — Phase 1 uses TS 6.0.3 per research R2)
- MASTER_PROMPT §2 Repository Structure (`apps/landing/`, `apps/extension/`, `packages/shared-types/`)
- MASTER_PROMPT §13 Phase 1 — "Create apps/* stubs" (line 2290)
- JS stack research §3 Next, §4 Svelte, §5 Vite+CRXJS, §7 TS, §11 shared-types, §8 CRXJS gotcha (`plans/reports/researcher-260424-0247-js-stack-versions.md`)
- Prior phase: `phase-01-monorepo-backbone.md` (pnpm workspace + tsconfig.base.json reference)

## Overview

**Priority:** P1 · **Status:** pending · **Effort:** 1.5h
Scaffold 3 workspace packages with package.json + minimal build config so `pnpm -r build` succeeds. NO UI code. NO Tailwind, NO obfuscator, NO WASM — those are Phase 3+ and 9. Only `page.tsx` + `layout.tsx` returning `"Snake Backlink Forge — landing placeholder"` for landing; minimal `manifest.config.ts` + empty SW `background/index.ts` for extension; `export {}` for shared-types.

## Key Insights

- **TypeScript 6.0.3** (research R2 §7) replaces "TS 5.5+" from ADR — more stable Biome 2 Svelte integration, compatible with Next.js 15.5.
- **Next.js 15.5** ships with React 19 stable — no need for canary tracking.
- **CRXJS v2.4.0** auto-infers entries from manifest; do NOT override `build.rollupOptions.inputs` (research §8 gotcha).
- **MV3 SW heartbeat** via `chrome.alarms` is Phase 3 concern (keeps SW alive after 30s idle) — Phase 1 SW just logs `console.log("sbf background boot")`.
- **shared-types** Phase 1: no tsc build; Turborepo typecheck is sufficient. `"types": "./src/index.ts"` direct source export.
- **tsconfig.base.json** at root provides monorepo path aliases `@sbf/shared-types/*`.

## Requirements

**Functional:**
- `apps/landing/` has package.json (Next 15.5 + React 19 + TS 6.0.3), `next.config.mjs`, `tsconfig.json`, `src/app/layout.tsx`, `src/app/page.tsx`. `pnpm --filter landing build` succeeds.
- `apps/extension/` has package.json (Vite 5 + @crxjs/vite-plugin 2.4.0 + Svelte 5 + TS 6.0.3), `vite.config.ts`, `tsconfig.json`, `src/manifest.config.ts`, `src/background/index.ts`. `pnpm --filter extension build` produces `dist/` with valid MV3 manifest.
- `packages/shared-types/` has package.json (TS only), `tsconfig.json`, `src/index.ts` containing `export {}`. `pnpm --filter @sbf/shared-types typecheck` clean.
- Root `tsconfig.base.json` — strict mode on, path aliases `@sbf/shared-types/*` → `packages/shared-types/src/*`.

**Non-functional:**
- All packages inherit from root `tsconfig.base.json` via `"extends"`.
- No Tailwind, no shadcn, no framer-motion (Phase 9).
- No content scripts, offscreen, popup UI, options UI (Phase 3).
- Each package < 200 LOC total for Phase 1 scope.

## Architecture

```
apps/
├── landing/
│   ├── package.json            (next 15.5, react 19, typescript workspace)
│   ├── next.config.mjs         (experimental: {}, no special flags)
│   ├── tsconfig.json           (extends ../../tsconfig.base.json + next plugin)
│   └── src/app/
│       ├── layout.tsx          (minimal html skeleton, "en" lang)
│       └── page.tsx            ("Snake Backlink Forge — landing placeholder" h1)
├── extension/
│   ├── package.json            (vite 5, @crxjs/vite-plugin 2.4.0, svelte 5, typescript workspace)
│   ├── vite.config.ts          (svelte plugin + crx plugin, no obfuscator)
│   ├── tsconfig.json
│   └── src/
│       ├── manifest.config.ts  (defineManifest: name, version, background SW, permissions:[] — Phase 3 adds storage/alarms/offscreen)
│       └── background/
│           └── index.ts        (console.log('sbf background boot'))
packages/
└── shared-types/
    ├── package.json            (name "@sbf/shared-types", main "src/index.ts")
    ├── tsconfig.json
    └── src/index.ts            (export {})

tsconfig.base.json              (strict true, module esnext, moduleResolution bundler, path aliases)
```

## Related Code Files

**CREATE:**
- `C:\Users\Hanna\Desktop\tool_backlink\tsconfig.base.json` (root)
- `apps/landing/package.json`
- `apps/landing/next.config.mjs`
- `apps/landing/tsconfig.json`
- `apps/landing/src/app/layout.tsx`
- `apps/landing/src/app/page.tsx`
- `apps/landing/.gitignore` (`/.next`, `/out`, `next-env.d.ts`)
- `apps/extension/package.json`
- `apps/extension/vite.config.ts`
- `apps/extension/tsconfig.json`
- `apps/extension/src/manifest.config.ts`
- `apps/extension/src/background/index.ts`
- `apps/extension/.gitignore` (`/dist`, `/.vite`)
- `packages/shared-types/package.json`
- `packages/shared-types/tsconfig.json`
- `packages/shared-types/src/index.ts`

**MODIFY:** none.

**DELETE:** none.

## Implementation Steps

1. Create root `tsconfig.base.json` with `compilerOptions`: strict true, module/target esnext, moduleResolution bundler, esModuleInterop true, skipLibCheck true, sourceMap true, baseUrl `.`, paths `@sbf/shared-types/*` → `packages/shared-types/src/*`.
2. Create `apps/landing/package.json` — `name: "@sbf/landing"`, private, scripts `build: next build`, `dev: next dev`, `lint: next lint`, `typecheck: tsc --noEmit`, deps `next@15.5.0 react@^19 react-dom@^19`, devDeps `@types/react@^19 @types/node@^20 typescript@workspace:* eslint-config-next@15.5.0`.
3. Create `apps/landing/next.config.mjs` — `export default { reactStrictMode: true };` (minimal).
4. Create `apps/landing/tsconfig.json` per **Next.js 15 official template** — `"extends": "../../tsconfig.base.json"`, `"compilerOptions": { "strict": true, "plugins": [{"name":"next"}], "jsx":"preserve", "incremental":true, "lib":["dom","dom.iterable","esnext"] }`, `"include": ["src/**/*", "next-env.d.ts", ".next/types/**/*.ts"]`, `"exclude": ["node_modules"]`. Note `strict:true` is defensive-explicit even though inherited from base.
5. Create `apps/landing/src/app/layout.tsx` — exports `metadata = {title:"Snake Backlink Forge"}` + default `RootLayout({children}: {children: React.ReactNode})` returning `<html lang="en"><body>{children}</body></html>`.
6. Create `apps/landing/src/app/page.tsx` — default export `HomePage()` returning `<h1>Snake Backlink Forge — landing placeholder</h1>`.
7. Create `apps/extension/package.json` — `name: "@sbf/extension"`, private, scripts `build: vite build`, `dev: vite`, `typecheck: tsc --noEmit`, deps `svelte@^5`, devDeps `vite@^5 @crxjs/vite-plugin@2.4.0 @sveltejs/vite-plugin-svelte@^3 typescript@workspace:* @types/chrome@latest svelte-check@latest`.
8. Create `apps/extension/vite.config.ts` — imports `defineConfig` from vite, `svelte` from `@sveltejs/vite-plugin-svelte`, `crx` from `@crxjs/vite-plugin`, default import `manifest` from `./src/manifest.config`; `export default defineConfig({ plugins: [svelte(), crx({ manifest })] })`. Do NOT override `build.rollupOptions.inputs` (CRXJS auto-infers).
9. Create `apps/extension/tsconfig.json` — extends base, `"compilerOptions": { "strict": true, "rootDir": "./src", "types": ["vite/client", "chrome"], "lib":["dom","esnext","webworker"], "jsx":"preserve" }`, includes `src/**/*`. Defensive-explicit `strict:true` + `rootDir` for clarity; `vite/client` for `import.meta.env`; `chrome` for MV3 APIs (svelte types loaded by svelte-check, not via tsconfig).
10. Create `apps/extension/src/manifest.config.ts` — import `defineManifest` from `@crxjs/vite-plugin`, default export with: `manifest_version: 3`, `name: "Snake Backlink Forge"`, `version: "0.1.0"`, `description`, `permissions: []`, `host_permissions: []`, `background: { service_worker: "src/background/index.ts", type: "module" }`. Omit `icons` Phase 1 (add Phase 3 with real PNGs). **[H8]** `permissions: []` empty for Phase 1 — offscreen/alarms/storage are unused dead permissions and may flag Chrome Web Store review. Add comment: `// Phase 3 will add: storage, alarms, offscreen, scripting, tabs, cookies, declarativeNetRequest per MASTER_PROMPT §6.1`.
11. Create `apps/extension/src/background/index.ts` — single statement: `console.log("[sbf] background boot", { ts: Date.now() });`.
12. Create `packages/shared-types/package.json` — `name: "@sbf/shared-types"`, private, version `0.1.0`, `main: "src/index.ts"`, `types: "src/index.ts"`, `scripts.typecheck: tsc --noEmit`, devDep `typescript@workspace:*`.
13. Create `packages/shared-types/tsconfig.json` — extends base, `"compilerOptions": { "strict": true, "noEmit": true, "types": [] }`, includes `src/**/*`. Defensive-explicit `strict:true` + empty `types:[]` (no ambient types leak into shared types package).
14. Create `packages/shared-types/src/index.ts`: `export {};`
15. `pnpm install` at root; verify `pnpm --filter @sbf/landing build` and `pnpm --filter @sbf/extension build` both exit 0.

## Todo List

- [ ] Root `tsconfig.base.json` (strict, path aliases)
- [ ] `apps/landing/package.json` + `next.config.mjs` + `tsconfig.json`
- [ ] `apps/landing/src/app/{layout,page}.tsx` placeholders
- [ ] `apps/extension/package.json` + `vite.config.ts` + `tsconfig.json`
- [ ] `apps/extension/src/manifest.config.ts` minimal MV3
- [ ] `apps/extension/src/background/index.ts` boot log
- [ ] `packages/shared-types/{package,tsconfig}.json`
- [ ] `packages/shared-types/src/index.ts` (export {})
- [ ] `pnpm install` + verify all 3 `build` / `typecheck` scripts pass

## Success Criteria

- `pnpm --filter @sbf/landing build` exit 0 → `apps/landing/.next/` populated
- `pnpm --filter @sbf/extension build` exit 0 → `apps/extension/dist/manifest.json` valid (parseable MV3), `dist/background/index.js` present
- `pnpm --filter @sbf/shared-types typecheck` exit 0
- `pnpm -r typecheck` exit 0 across all 3 packages
- `turbo run build --dry-run` prints exactly 2 build tasks (landing + extension) — shared-types has no build task
- Root `tsconfig.base.json` parsed: `npx tsc -p tsconfig.base.json --noEmit --showConfig | jq .compilerOptions.strict` returns `true`

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| CRXJS build fails on empty icon paths | Med | Low | Pass empty strings (plugin tolerates) OR drop `icons` block Phase 1 → re-add Phase 3 with actual PNGs |
| Next 15.5 requires Node >= 20 — conflicts with user's Node 18 | Low | Med | Document in README: `node >= 20`; CI pins Node 20 |
| Svelte 5 + TS 6.0 strict `this: void` errors on callbacks | Low | Low | None yet (no Svelte UI code Phase 1); address Phase 3 |
| **[H6]** TS 6.0.3 + React 19 + Next.js 15.5 type compat — `@types/react@^19` with TS 6 may emit errors on prerelease type defs | Med | Med | Phase 1 skeleton build is the validation gate; if fails, downgrade to `typescript: 5.6` via per-workspace `tsconfig.json` override in `apps/landing` |
| @crxjs/vite-plugin 2.4.0 requires `import.meta.url` features not polyfilled by Vite 5 | Low | Low | Document; if breaks, downgrade to 2.3.x — but research confirms 2.4.0 stable |

## Security Considerations

- `apps/extension/vite.config.ts` has NO obfuscator Phase 1 — raw JS in `dist/` is acceptable since no business logic yet. Obfuscator added Phase 7 per §6.9.
- `host_permissions: []` — no `<all_urls>` yet. Added Phase 4 when adapters need page DOM access.
- `background/index.ts` has no network calls — no CSP surface.
- `manifest.version: "0.1.0"` — bump per release; Phase 10 CI automates.

## Next Steps

Unblocks Phase 06 (CI + lint needs `apps/*` and `packages/*` to exist; Biome config scopes these globs). Does NOT block Phase 07 directly. Phase 3 fills extension (WASM, offscreen, popup). Phase 9 fills landing (hero, pricing, blog).
