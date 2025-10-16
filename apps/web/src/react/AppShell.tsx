import * as React from "react";
import { LanguageProvider } from "./LanguageProvider";
import Footer from "./Footer";
import { Toaster } from "@/components/ui/toaster";
import { gql } from "@/lib/graphql";

export default function AppShell() {
  React.useEffect(() => {
    // Only run on protected routes to avoid redirect loops
    const path = location.pathname;
    const isProtected = ["/home", "/settings", "/admin"].some(
      (p) => path === p || path.startsWith(p + "/")
    );
    if (!isProtected) return;
    (async () => {
      try {
        const me = await gql<{ me: { is_agreed_policies: boolean } }>(
          "query{ me{ is_agreed_policies } }"
        );
        if (!me.me?.is_agreed_policies) {
          const next = encodeURIComponent(
            location.pathname + location.search + location.hash
          );
          location.replace(`/consent?next=${next}`);
        }
      } catch {
        // ignore errors (likely unauth)
      }
    })();
  }, []);
  return (
    <LanguageProvider>
      <Toaster />
      <Footer />
    </LanguageProvider>
  );
}
