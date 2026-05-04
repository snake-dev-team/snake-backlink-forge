import { CodeWindow } from "@/components/ui/code-window";

/**
 * Hero right-top: Telegram bot CLI snippet wrapped in Phase 03 CodeWindow.
 * Shows 3-line bash session: comment → bot command → mock status output.
 * CodeWindow runs as async RSC — Shiki highlights at build time, zero client JS.
 */
const HERO_CLI = `# Nạp 100 credit, kết nối WordPress trong 60s
@SnakeBacklinkForgeBot /start
> Telegram credit: 100  · Sites: 0 · Campaigns: 0`;

export function HeroCliSnippet() {
  return <CodeWindow title="telegram" lang="bash" code={HERO_CLI} className="glass-card-strong" />;
}
