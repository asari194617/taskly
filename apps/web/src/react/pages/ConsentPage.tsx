"use client";
import * as React from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { gql } from "@/lib/graphql";
import { useLang } from "@/react/LanguageProvider";

function PolicyLink({ href, label, lang, agree }: { href: string; label: string; lang: "ja" | "en"; agree: string }) {
  const link = (
    <a href={href} target="_blank" className="underline">
      {label}
    </a>
  );
  return lang === "ja" ? <>{link}{agree}</> : <>{agree} {link}</>;
}

export default function ConsentPage() {
  const { dict, lang } = useLang();
  const [tos, setTos] = React.useState(false);
  const [pp, setPp] = React.useState(false);
  const [sp, setSp] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const canSubmit = tos && pp && sp;

  const submit = async () => {
    if (!canSubmit) return;
    setLoading(true);
    try {
      await gql<{ agreePolicies: boolean }>("mutation{ agreePolicies }");
      const next = new URLSearchParams(location.search).get("next");
      location.replace(next && next.startsWith("/") ? next : "/home");
    } catch {
      // silently ignore; user can retry
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card className="p-6 space-y-4">
      <h1 className="text-2xl font-semibold">{dict.consent.heading}</h1>
      <p className="text-sm text-muted-foreground">
        {dict.consent.description}
      </p>
      <div className="space-y-3">
        <label
          className="flex items-center gap-3"
          onClick={(e) => {
            // If the user clicked the inner anchor, let it navigate normally
            const tgt = e.target as HTMLElement;
            if (tgt && (tgt.tagName === "A" || tgt.closest("a"))) return;
            setTos((s) => !s);
          }}
        >
          <Checkbox checked={tos} />
          <span>
            <PolicyLink
              href="https://ritzy-lift-e38.notion.site/28ba3874f8c880a783ddf9d39aea1242?source=copy_link"
              label={dict.common.terms}
              lang={lang}
              agree={dict.consent.agree}
            />
          </span>
        </label>
        <label
          className="flex items-center gap-3"
          onClick={(e) => {
            const tgt = e.target as HTMLElement;
            if (tgt && (tgt.tagName === "A" || tgt.closest("a"))) return;
            setPp((s) => !s);
          }}
        >
          <Checkbox checked={pp} />
          <span>
            <PolicyLink
              href="https://ritzy-lift-e38.notion.site/28ba3874f8c88073a4b2c0a56b82bc03?source=copy_link"
              label={dict.common.privacyPolicy}
              lang={lang}
              agree={dict.consent.agree}
            />
          </span>
        </label>
        <label
          className="flex items-center gap-3"
          onClick={(e) => {
            const tgt = e.target as HTMLElement;
            if (tgt && (tgt.tagName === "A" || tgt.closest("a"))) return;
            setSp((s) => !s);
          }}
        >
          <Checkbox checked={sp} />
          <span>
            <PolicyLink
              href="https://ritzy-lift-e38.notion.site/28ba3874f8c880938951dab0cbd4814f?source=copy_link"
              label={dict.common.securityPolicy}
              lang={lang}
              agree={dict.consent.agree}
            />
          </span>
        </label>
      </div>
      <div className="pt-2">
        <Button onClick={submit} disabled={!canSubmit || loading}>
          {dict.consent.continue}
        </Button>
      </div>
    </Card>
  );
}
