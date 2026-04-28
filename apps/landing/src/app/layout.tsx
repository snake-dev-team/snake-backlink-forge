import "./globals.css";
import type { Metadata } from "next";
import type { ReactNode } from "react";
import { ThemeProvider } from "@/components/providers/theme-provider";
import { PlausibleScript } from "@/lib/analytics/plausible";

export const metadata: Metadata = {
  title: "Snake Backlink Forge",
  description: "Professional SEO backlink automation platform.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="vi" suppressHydrationWarning>
      <body>
        <ThemeProvider>{children}</ThemeProvider>
        <PlausibleScript />
      </body>
    </html>
  );
}
