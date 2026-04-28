import { ArrowRight, Bot, Code2, KeyRound, Sparkles, Terminal, Zap } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

const botUsername = process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME ?? "SnakeBacklinkForgeBot";

const features = [
  {
    title: "AI tạo bài viết chuẩn SEO",
    description:
      "Sinh nội dung tiếng Việt có cấu trúc, anchor rõ ràng và đủ ngữ cảnh để dùng cho chiến dịch backlink.",
    icon: Sparkles,
  },
  {
    title: "Đăng tự động lên WordPress",
    description:
      "Kết nối bằng Application Password, validate REST API và quyền publish_posts trước khi lưu site.",
    icon: Zap,
  },
  {
    title: "Phân tích từ khóa thông minh",
    description:
      "Gợi ý intent, cụm từ liên quan và nhịp đăng phù hợp để tránh spam footprint khi scale.",
    icon: Terminal,
  },
];

const pricing = [
  { name: "Starter", price: "$30", detail: "Test flow, ít site, volume thấp" },
  { name: "Growth", price: "$59", detail: "Đội SEO nhỏ, chạy đều mỗi tuần" },
  { name: "Scale", price: "$99", detail: "Nhiều site, cần tự động hóa mạnh hơn" },
];

export default function HomePage() {
  return (
    <main id="main" tabIndex={-1} className="min-h-screen overflow-hidden bg-[#12091f] text-white">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:rounded-full focus:bg-cyan-300 focus:px-4 focus:py-2 focus:text-sm focus:font-semibold focus:text-slate-950"
      >
        Bỏ qua tới nội dung chính
      </a>

      <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_top_left,rgba(124,58,237,0.34),transparent_34%),radial-gradient(circle_at_top_right,rgba(6,182,212,0.22),transparent_30%),linear-gradient(180deg,rgba(18,9,31,0),#12091f_72%)]" />
      <div className="pointer-events-none fixed inset-0 bg-[linear-gradient(rgba(255,255,255,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.035)_1px,transparent_1px)] bg-[size:72px_72px] opacity-25" />

      <div className="relative mx-auto flex w-full max-w-7xl flex-col px-5 py-6 sm:px-8 lg:px-10">
        <nav className="flex items-center justify-between rounded-full border border-white/10 bg-white/[0.04] px-4 py-3 shadow-2xl shadow-violet-950/30 backdrop-blur md:px-5">
          <Link href="/" className="flex items-center gap-2 text-sm font-semibold tracking-tight">
            <span className="flex size-8 items-center justify-center rounded-full bg-cyan-300 text-slate-950">
              <Code2 className="size-4" aria-hidden="true" />
            </span>
            Snake Backlink Forge
          </Link>
          <div className="hidden items-center gap-6 text-sm text-white/70 md:flex">
            <a href="#features" className="hover:text-cyan-200">
              Tính năng
            </a>
            <a href="#pricing" className="hover:text-cyan-200">
              Giá
            </a>
            <a href="#contact" className="hover:text-cyan-200">
              Liên hệ
            </a>
          </div>
          <Button
            asChild
            size="sm"
            className="rounded-full bg-white text-slate-950 hover:bg-cyan-100"
          >
            <Link href="/login">Đăng nhập</Link>
          </Button>
        </nav>

        <section className="grid gap-12 py-20 lg:grid-cols-[1.02fr_0.98fr] lg:items-center lg:py-28">
          <div className="space-y-8">
            <div className="inline-flex items-center gap-2 rounded-full border border-cyan-300/20 bg-cyan-300/10 px-3 py-1 text-sm text-cyan-100">
              <Bot className="size-4" aria-hidden="true" />
              SEO automation stack cho team thích dashboard rõ ràng
            </div>
            <div className="space-y-5">
              <h1 className="max-w-4xl text-5xl font-semibold tracking-[-0.05em] text-white sm:text-6xl lg:text-7xl">
                Tạo, kiểm soát và đăng backlink WordPress từ một cockpit.
              </h1>
              <p className="max-w-2xl text-lg leading-8 text-white/68">
                Snake Backlink Forge gom API key, credit, AI content và WordPress publishing vào một
                luồng vận hành gọn cho SEO operator.
              </p>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row">
              <Button
                asChild
                size="lg"
                className="rounded-full bg-cyan-300 text-slate-950 hover:bg-cyan-200"
              >
                <a href={`https://t.me/${botUsername}`} target="_blank" rel="noopener noreferrer">
                  Mở Telegram bot
                  <ArrowRight className="size-4" aria-hidden="true" />
                </a>
              </Button>
              <Button
                asChild
                size="lg"
                variant="outline"
                className="rounded-full border-white/15 bg-white/5 text-white hover:bg-white/10 hover:text-white"
              >
                <a href="#pricing">Xem giá</a>
              </Button>
            </div>
          </div>

          <div className="rounded-[2rem] border border-white/10 bg-slate-950/65 p-4 shadow-2xl shadow-cyan-950/30 backdrop-blur">
            <div className="rounded-[1.5rem] border border-white/10 bg-[#0b1020] p-5">
              <div className="mb-5 flex items-center justify-between border-b border-white/10 pb-4">
                <div className="flex gap-2">
                  <span className="size-3 rounded-full bg-rose-400" />
                  <span className="size-3 rounded-full bg-amber-300" />
                  <span className="size-3 rounded-full bg-emerald-300" />
                </div>
                <span className="text-xs text-white/40">/campaigns/run</span>
              </div>
              <div className="space-y-4 font-mono text-sm text-white/76">
                <p>
                  <span className="text-cyan-300">const</span> campaign = createBacklinkRun()
                </p>
                <p className="pl-4 text-white/55">site.connect("wordpress", status: "validated")</p>
                <p className="pl-4 text-white/55">ai.write(topic, intent, anchors)</p>
                <p className="pl-4 text-white/55">publisher.queue(drip: "safe")</p>
                <div className="rounded-2xl border border-cyan-300/20 bg-cyan-300/10 p-4 font-sans">
                  <div className="flex items-center gap-2 text-cyan-100">
                    <KeyRound className="size-4" aria-hidden="true" />
                    API key verified · credits ready · WP REST OK
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section id="features" className="grid gap-4 py-10 md:grid-cols-3">
          {features.map((feature) => (
            <Card
              key={feature.title}
              className="border-white/10 bg-white/[0.04] text-white shadow-none backdrop-blur"
            >
              <CardHeader>
                <div className="mb-4 flex size-11 items-center justify-center rounded-2xl bg-violet-400/15 text-cyan-200">
                  <feature.icon className="size-5" aria-hidden="true" />
                </div>
                <CardTitle className="text-xl leading-7">{feature.title}</CardTitle>
              </CardHeader>
              <CardContent className="text-sm leading-7 text-white/62">
                {feature.description}
              </CardContent>
            </Card>
          ))}
        </section>

        <section id="pricing" className="py-16">
          <div className="mb-8 max-w-2xl space-y-3">
            <p className="text-sm font-medium uppercase tracking-[0.24em] text-amber-200">
              Pricing teaser
            </p>
            <h2 className="text-3xl font-semibold tracking-tight sm:text-4xl">
              Bắt đầu nhỏ, scale khi workflow ổn.
            </h2>
          </div>
          <div className="grid gap-4 md:grid-cols-3">
            {pricing.map((plan) => (
              <Card
                key={plan.name}
                className="border-white/10 bg-white/[0.05] text-white shadow-none"
              >
                <CardHeader>
                  <CardTitle className="flex items-end justify-between gap-4">
                    <span>{plan.name}</span>
                    <span className="text-3xl text-cyan-200">{plan.price}</span>
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-5 text-sm text-white/62">
                  <p>{plan.detail}</p>
                  <Button
                    asChild
                    variant="outline"
                    className="w-full rounded-full border-white/15 bg-white/5 text-white hover:bg-white/10 hover:text-white"
                  >
                    <a
                      href={`https://t.me/${botUsername}`}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Liên hệ qua bot
                    </a>
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        </section>

        <footer
          id="contact"
          className="flex flex-col gap-4 border-t border-white/10 py-8 text-sm text-white/55 md:flex-row md:items-center md:justify-between"
        >
          <p>© 2026 Snake Backlink Forge. SEO automation cho operator Việt Nam.</p>
          <div className="flex gap-4">
            <a
              className="hover:text-cyan-200"
              href={`https://t.me/${botUsername}`}
              target="_blank"
              rel="noopener noreferrer"
            >
              Telegram
            </a>
            <a className="hover:text-cyan-200" href="mailto:support@snakepremiumhub.com">
              support@snakepremiumhub.com
            </a>
          </div>
        </footer>
      </div>
    </main>
  );
}
