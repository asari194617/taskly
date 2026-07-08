"use client";
import * as React from "react";
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "@/components/ui/hover-card";
import { MapPin, Clock, Contact } from "lucide-react";

type LinkItem = { title: string; href: string };
import { useLang } from "./LanguageProvider";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
type Section = { title: string; links: LinkItem[] };

export default function Footer() {
  const [timeStr, setTimeStr] = React.useState<string>(formatJST());
  const { lang, setLang, dict } = useLang();
  const d = dict.common;

  const SECTIONS: Section[] = React.useMemo(
    () => [
      {
        title: d.products,
        links: [
          { title: d.home, href: "/" },
          { title: d.settings, href: "/settings" },
          // { title: d.admin, href: "/admin" },
          // { title: d.contact, href: "mailto:" },
        ],
      },
      {
        title: d.documentation,
        links: [
          { title: d.guide, href: "/guide" },
          { title: d.api, href: "https://cdn.taskly.iy0306.com/docs/api/index.html" },
          { title: d.openapiFile, href: "https://cdn.taskly.iy0306.com/docs/api/index.html" },
          { title: d.calendar, href: "/guide/calendar" },
          { title: d.mcpServer, href: "/guide/mcp" },
        ],
      },
      {
        title: d.platform,
        links: [
          { title: "Web", href: "https://taskly.iy0306.com/" },
          {
            title: "iPhone,iPad,Mac(近日公開)",
            href: "https://taskly.iy0306.com/platform/iphone-ipad-mac",
          },
          {
            title: "Android(近日公開)",
            href: "https://taskly.iy0306.com/platform/android",
          },
          {
            title: "Windows(近日公開)",
            href: "https://taskly.iy0306.com/platform/windows",
          },
        ],
      },
      {
        title: d.other,
        links: [
          {
            title: d.terms,
            href: "https://ritzy-lift-e38.notion.site/28ba3874f8c880a783ddf9d39aea1242",
          },
          {
            title: d.privacyPolicy,
            href: "https://ritzy-lift-e38.notion.site/28ba3874f8c88073a4b2c0a56b82bc03",
          },
          {
            title: d.securityPolicy,
            href: "https://ritzy-lift-e38.notion.site/28ba3874f8c880938951dab0cbd4814f",
          },
          {
            title: d.continuityStatement,
            href: "https://ritzy-lift-e38.notion.site/28ba3874f8c8800bac93e03c6dec44c8",
          },
          { title: "GitHub", href: "https://github.com/asari194617/taskly" },
        ],
      },
    ],
    [d]
  );

  React.useEffect(() => {
    const id = setInterval(() => setTimeStr(formatJST()), 30_000);
    return () => clearInterval(id);
  }, []);

  return (
    <footer className="border-t bg-background text-foreground">
      <div className="container py-10 mx-auto px-2">
        <div className="grid grid-cols-1 gap-8 @xs:grid-cols-2 sm:grid-cols-3 lg:grid-cols-4">
          {SECTIONS.map((sec) => (
            <div key={sec.title} className="space-y-3">
              <div className="text-sm font-semibold text-foreground/90 dark:text-foreground">
                {sec.title}
              </div>
              <ul className="space-y-2 text-sm text-muted-foreground">
                {sec.links.map((l) => (
                  <li key={l.title}>
                    <a
                      href={l.href}
                      className="hover:text-foreground transition-colors"
                    >
                      {l.title}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="mt-10 border-t pt-6 text-xs sm:text-sm text-muted-foreground flex flex-col items-start gap-3">
          <div className="flex items-center gap-2 text-sm">
            <span className="text-foreground/80">{d.language}:</span>
            <div className="min-w-[160px]">
              <Select
                value={lang}
                onValueChange={(v: any) => {
                  console.log("[Footer] Select onValueChange:", v);
                  setLang(v);
                }}
              >
                <SelectTrigger aria-label={d.language}>
                  <SelectValue placeholder={d.language} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ja">日本語</SelectItem>
                  <SelectItem value="en">English</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="flex items-center">
            <span>Made with❣️ by&nbsp;</span>
            <HoverCard>
              <HoverCardTrigger asChild>
                <a
                  href="https://github.com/asari194617"
                  target="_blank"
                  rel="noreferrer noopener"
                  className="underline-offset-4 hover:underline text-foreground"
                >
                  asari
                </a>
              </HoverCardTrigger>
              <HoverCardContent className="w-80" align="start">
                <div className="space-y-3">
                  <div className="flex items-center gap-2">
                    <img
                      loading="lazy"
                      width="322"
                      height="322"
                      style={{ width: 32, height: 32, borderRadius: "50%" }}
                      alt="asari194617のアイコン"
                      src="https://cdn.taskly.iy0306.com/assets/asari-logo.webp"
                      className="w-full rounded-md border"
                    />
                    <div className="text-lg font-semibold">asari</div>
                  </div>
                  <div className="flex items-center gap-2">
                    <MapPin className="h-4 w-4" />
                    <span className="text-sm">Tokyo, Japan</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <Clock className="h-4 w-4" />
                    <span className="text-sm">{timeStr} (UTC +09:00)</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <Contact className="h-4 w-4" />
                    <a
                      href="https://x.com/asari194617"
                      target="_blank"
                      rel="noreferrer noopener"
                      className="text-sm underline-offset-4 hover:underline"
                    >
                      @asari194617
                    </a>
                  </div>
                </div>
              </HoverCardContent>
            </HoverCard>
            <span>.</span>
          </div>
        </div>
      </div>
    </footer>
  );
}

function formatJST() {
  try {
    return new Intl.DateTimeFormat("ja-JP", {
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
      timeZone: "Asia/Tokyo",
    }).format(new Date());
  } catch {
    const d = new Date();
    const hh = String(d.getHours()).padStart(2, "0");
    const mm = String(d.getMinutes()).padStart(2, "0");
    return `${hh}:${mm}`;
  }
}
