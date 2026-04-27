// Ambient declarations for side-effect CSS imports in Next.js App Router.
// `next-env.d.ts` plus the Next.js TS plugin handle this at runtime, but standalone
// `tsc --noEmit` invocations (CI, pre-commit) need an explicit module declaration.
declare module "*.css";
