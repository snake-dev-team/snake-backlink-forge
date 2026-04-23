# JS/TS Stack Versions & Gotchas for Phase 1 — Snake Backlink Forge
**Research Date:** 2026-04-24  
**Phase Scope:** Foundation setup only (stubs, no UI implementation)

---

## 1. pnpm Latest Stable

**Version:** `9.15.9` (latest 9.x), but `10.33.2` current stable overall.  
**Recommendation for Phase 1:** Pin `9.15.9` for stability; monorepo features fully mature.  
**`pnpm-workspace.yaml` syntax:**
```yaml
packages:
  - 'apps/*'
  - 'packages/*'
  - 'services/*'
  - 'tools/*'
```
**`package.json` packageManager field:** `"packageManager": "pnpm@9.15.9"`  
**Source:** https://github.com/pnpm/pnpm/releases, https://endoflife.date/pnpm

---

## 2. Turborepo Latest 2.x

**Version:** `2.9.6` (latest, released March 30, 2026).  
**v2 schema highlights:**
- `tasks.{name}.dependsOn`: array of upstream tasks (e.g., `["^build", "lint"]`)
- `env`: root-level + task-level env vars (v2 strict by default)
- `cacheDir`: custom cache location (default `.turbo`)
- `outputs`: artifact globs for caching (e.g., `["dist/**", "build/**"]`)
- Global task pipeline (no separate task graphs per app)

**Phase 1 pipeline (recommended):**
```json
{
  "tasks": {
    "build": { "outputs": ["dist/**", "build/**"], "cache": true },
    "lint": { "cache": false },
    "typecheck": { "cache": false },
    "test": { "cache": false, "dependsOn": ["build"] },
    "dev": { "cache": false },
    "clean": { "cache": false }
  }
}
```
**Source:** https://github.com/vercel/turborepo/releases, https://turborepo.dev/blog/2-9

---

## 3. Next.js 15 Latest Minor

**Version:** `15.5` (latest minor as of April 2026).  
**TS Strict Config:** Add to `tsconfig.json`:
```json
{
  "compilerOptions": {
    "strict": true,
    "noImplicitAny": true,
    "strictNullChecks": true,
    "strictFunctionTypes": true,
    "strictBindCallApply": true,
    "strictPropertyInitialization": true,
    "noImplicitThis": true,
    "alwaysStrict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true
  }
}
```
**Config file:** `next.config.mjs` (`.js` works but `.mjs` explicit for ESM; Phase 1 landing skeleton doesn't need env vars yet).  
**React pairing:** React 19 stable shipping with Next.js 15.

**Source:** https://nextjs.org/docs/app/guides/upgrading/version-15, https://github.com/vercel/next.js/releases

---

## 4. Svelte 5 Runes Status

**Version:** `5.0.0+` (stable, shipped October 2024, fully production-ready as of April 2026).  
**Runes stable:** Yes — `$state`, `$derived`, `$effect`, `$props` are first-class reactivity primitives.  
**Breaking from 4.x:** Opt-in (existing components work), but new code should use runes syntax.  
**TS integration with Vite 5:** Full support. No gotchas; Svelte 5 ships with built-in TS support.

**Source:** https://svelte.dev/docs/svelte/v5-migration-guide, https://svelte.dev/blog/svelte-5-is-alive

---

## 5. Vite 5 Latest & @crxjs/vite-plugin

**Vite:** `5.x` stable, compatible with Node.js 16.11+ (use Node 18+ for Phase 1).  
**@crxjs/vite-plugin:** `2.4.0` (latest, published March 2026).  
**MV3 SW Lifecycle Gotcha:** Service Worker idles after 30s of inactivity. Phase 3 (extension) must implement `chrome.alarms` heartbeat every 25-30s to keep it alive. This is MV3-specific (MV2 had persistent SW).

**Source:** https://www.npmjs.com/package/@crxjs/vite-plugin, https://crxjs.dev/vite-plugin/

---

## 6. Biome (Formatter + Linter)

**Version:** `2.3` (v2.0 released March 2025; current as of Jan 2026).  
**Monorepo config:** Root `biome.json` with nested config support via `"extends": ["//"]` syntax.  
**Scope for Phase 1:** `*.ts`, `*.tsx`, `*.svelte`, `*.json` (Svelte support stable in v2).  
**Svelte support:** Stable as of v2. Biome auto-detects `.svelte` files.  
**Config skeleton:**
```json
{
  "organizeImports": { "enabled": true },
  "formatter": { "indentStyle": "space", "indentSize": 2 },
  "linter": {
    "enabled": true,
    "rules": { "recommended": true }
  },
  "overrides": [
    {
      "include": ["**/*.svelte"],
      "formatter": { "enabled": true }
    }
  ]
}
```
**Performance:** 10,000 files linted in 0.8s (vs ESLint 45.2s).

**Source:** https://biomejs.dev/, https://biomejs.dev/blog/roadmap-2026

---

## 7. TypeScript Latest

**Version:** `6.0` (released March 23, 2026) is current production; TypeScript 7.0 Beta available (Go rewrite, ~10x faster).  
**Recommendation for Phase 1:** Pin `6.0.x` for stability. TypeScript 7 beta not production-ready.  
**Strict mode:** Enable via `"strict": true` in `tsconfig.json`.  
**`tsconfig.base.json` pattern (monorepo):**
```json
{
  "compilerOptions": {
    "strict": true,
    "module": "esnext",
    "target": "esnext",
    "moduleResolution": "bundler",
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "baseUrl": ".",
    "paths": {
      "@shared-types/*": ["packages/shared-types/src/*"],
      "@landing/*": ["apps/landing/src/*"],
      "@extension/*": ["apps/extension/src/*"]
    }
  }
}
```

**Source:** https://github.com/microsoft/TypeScript/releases, https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html

---

## 8. @crxjs/vite-plugin MV3 Compat

**Latest:** `2.4.0`, last published ~1 month ago (March 2026).  
**Breaking changes last 6 months:** No major breaking changes reported. Plugin remains stable for MV3.  
**Known gotcha:** Do NOT add `build.rollupOptions.inputs` manually — CRXJS auto-infers entry points from manifest. Custom inputs will cause build collision.

**Source:** https://github.com/crxjs/chrome-extension-tools/issues/876

---

## 9. Gitleaks Latest

**Version:** `8.18.2+` (actively maintained, integrates with pre-commit and Husky).  
**Husky integration pattern (Phase 1):**
```bash
# .husky/pre-commit
gitleaks detect --verbose --log-opts -1 -S
```
**Alternative:** Use pre-commit framework `.pre-commit-config.yaml`:
```yaml
- repo: https://github.com/gitleaks/gitleaks
  rev: v8.18.2
  hooks:
    - id: gitleaks
```

**Source:** https://github.com/gitleaks/gitleaks, https://www.d4b.dev/blog/2026-02-01-gitleaks-pre-commit-hook/

---

## 10. Commitlint Latest

**Version:** `@commitlint/config-conventional@20.5.0` (latest, published March 2026).  
**`.commitlintrc.json` pattern (Phase 1):**
```json
{
  "extends": ["@commitlint/config-conventional"],
  "rules": {
    "type-enum": ["error", "always", ["feat", "fix", "docs", "test", "refactor", "chore", "perf"]],
    "scope-case": ["error", "always", "kebab-case"],
    "subject-case": ["error", "always", "lower-case"],
    "subject-full-stop": ["error", "never", "."]
  }
}
```
**Preset:** `conventional-commits` (via `@commitlint/config-conventional`).

**Source:** https://github.com/conventional-changelog/commitlint, https://commitlint.js.org/

---

## 11. Shared-Types Package Pattern

**Recommended for Phase 1 (stub only):**
```
packages/shared-types/
├── src/
│   ├── index.ts            # Export all types
│   ├── api.ts              # API request/response types (empty stubs for Phase 1)
│   ├── extension.ts        # Extension messaging types (empty stubs)
│   ├── campaign.ts         # Campaign/job types (empty stubs)
│   └── telegram.ts         # Telegram bot types (empty stubs)
├── tsconfig.json           # Extends ../../tsconfig.base.json
└── package.json            # { "main": "src/index.ts", "types": "src/index.ts" }
```
**Build approach:** Keep source TypeScript in repo for Phase 1; no tsc build step yet (Turborepo's TS checking sufficient). Add tsup build only in Phase 2+ when publishing.

**Source:** https://nx.dev/blog/managing-ts-packages-in-monorepos, https://colinhacks.com/essays/live-types-typescript-monorepo

---

## 12. Go → TypeScript Types (Phase 2+ Planning)

**Recommended tool:** `tygo` (handles Go structs → TS interfaces, preserves comments, supports constants).  
**Alternative:** `go2ts` (web UI for one-off conversions); `typescriptify-golang-structs` (reflection-based, mature).

**Phase 1 approach:** Leave `packages/shared-types` empty (or with placeholder stubs). Phase 2 will auto-gen from Go API schema via `tygo` in build pipeline.

**Source:** https://github.com/gzuidhof/tygo, https://github.com/StirlingMarketingGroup/go2ts

---

## Critical Gotchas

1. **Turborepo 2.x env strictness:** Tasks inherit root env, but explicit `env: []` disables inheritance. Set task-level env vars explicitly if needed (avoid surprises in CI).

2. **MV3 Service Worker idle:** Extension must implement `chrome.alarms` heartbeat ≤30s or SW will pause. This kills background loops. Ensure Phase 3 extension skeleton includes alarm handler skeleton.

3. **CRXJS + Rollup inputs:** Do NOT override `build.rollupOptions.inputs` — CRXJS fails at build time. Let plugin infer from manifest.

4. **Biome + Svelte:** First time linting Svelte with Biome? Requires explicit overrides in config. Auto-format may reorder JSX props in .svelte files — acceptable for Phase 1, fine-tune in Phase 2.

5. **TypeScript 6.0 no-implicit-this:** If using callback fns in extensions/adapters, add `this: void` or explicit `this: SomeType` to avoid strict mode grief.

---

## Recommended Root package.json devDependencies

```json
{
  "devDependencies": {
    "pnpm": "9.15.9",
    "turbo": "2.9.6",
    "typescript": "6.0.3",
    "@typescript-eslint/eslint-plugin": "^7.0.0",
    "@typescript-eslint/parser": "^7.0.0",
    "biome": "2.3.0",
    "prettier": "3.2.5",
    "@commitlint/cli": "^20.5.0",
    "@commitlint/config-conventional": "^20.5.0",
    "husky": "^9.0.0",
    "lint-staged": "^15.0.0",
    "gitleaks": "8.18.2"
  }
}
```

---

## Recommended Per-App Packages (Phase 1 Stubs)

### apps/landing (Next.js 15.5)
```json
{
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "next": "15.5.0",
    "framer-motion": "^11.0.0",
    "tailwindcss": "^4.0.0"
  },
  "devDependencies": {
    "typescript": "workspace:*",
    "@types/react": "^19.0.0",
    "@types/node": "^20.0.0",
    "eslint": "^9.0.0",
    "eslint-config-next": "15.5.0"
  }
}
```

### apps/extension (Svelte 5 + Vite + CRXJS)
```json
{
  "dependencies": {
    "svelte": "^5.0.0",
    "tailwindcss": "^4.0.0"
  },
  "devDependencies": {
    "vite": "^5.0.0",
    "@crxjs/vite-plugin": "2.4.0",
    "svelte": "^5.0.0",
    "typescript": "workspace:*",
    "@sveltejs/vite-plugin-svelte": "^3.0.0",
    "tailwindcss": "^4.0.0"
  }
}
```

### packages/shared-types
```json
{
  "devDependencies": {
    "typescript": "workspace:*"
  }
}
```

---

## Unresolved Questions

1. **Biome Svelte CSS:** Does Biome auto-format `<style>` blocks in `.svelte`? (Likely yes in v2, but verify in Phase 3 extension setup.)

2. **WASM+CSP in extension:** Will CRXJS 2.4.0 auto-generate correct CSP `wasm-unsafe-eval` in manifest? (Likely, but confirm in Phase 3.)

3. **Go type generation timing:** Should `tygo` run in Turborepo task pipeline (Phase 2) or as separate CI step? (Recommend Turborepo task for consistency.)

4. **Svelte runes + Vite SSR:** Phase 1 is no-SSR (landing is CSR via Next.js, extension is extension). No gotcha expected, but revisit in Phase 2+ if needed.

5. **TypeScript 6.0 strict + @crxjs/vite-plugin:** Any type incompatibilities? (No reports found; likely compatible.)
