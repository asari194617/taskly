import * as React from "react";
import { LanguageProvider, useLang } from "./LanguageProvider";
import Footer from "./Footer";
import { Toaster } from "@/components/ui/toaster";
import { gql } from "@/lib/graphql";

function PolicyNotice({ effectiveAt }: { effectiveAt: string }) {
  const { lang, dict } = useLang();
  const date = new Date(effectiveAt).toLocaleDateString(lang === "ja" ? "ja-JP" : "en-US");
  return (
    <aside role="status" className="fixed top-0 z-50 w-full bg-amber-100 px-4 py-3 text-center text-sm text-amber-950">
      {dict.consent.notice(date)}
      {" "}
      <a className="underline" href={`/consent?next=${encodeURIComponent(location.pathname + location.search)}`}>
        {dict.consent.noticeAction}
      </a>
    </aside>
  );
}

export default function AppShell() {
  const [policyNotice, setPolicyNotice] = React.useState("");
  React.useEffect(() => {
    // Only run on protected routes to avoid redirect loops
    const path = location.pathname;
    const isProtected = ["/home", "/tasks", "/settings", "/admin"].some(
      (p) => path === p || path.startsWith(p + "/")
    );
    if (!isProtected) return;
    (async () => {
      try {
        const me = await gql<{ me: { is_agreed_policies: boolean; policy_notice_effective_at: string } }>(
          "query{ me{ is_agreed_policies policy_notice_effective_at } }"
        );
        if (!me.me?.is_agreed_policies) {
          const next = encodeURIComponent(
            location.pathname + location.search + location.hash
          );
          location.replace(`/consent?next=${next}`);
        } else if (me.me.policy_notice_effective_at) {
          setPolicyNotice(me.me.policy_notice_effective_at);
        }
      } catch {
        // ignore errors (likely unauth)
      }
    })();
  }, []);
  return (
    <LanguageProvider>
      {policyNotice && <PolicyNotice effectiveAt={policyNotice} />}
      <Toaster />
      <Footer />
    </LanguageProvider>
  );
}
