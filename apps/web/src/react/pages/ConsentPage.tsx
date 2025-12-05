"use client";
import * as React from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { gql } from "@/lib/graphql";

export default function ConsentPage() {
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
      <h1 className="text-2xl font-semibold">ご利用にあたって</h1>
      <p className="text-sm text-muted-foreground">
        以下のポリシーに同意の上、進んでください。
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
          <Checkbox checked={tos} onCheckedChange={(v) => setTos(!!v)} />
          <span>
            <a href="/terms" target="_blank" className="underline">
              利用規約
            </a>
            に同意します
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
          <Checkbox checked={pp} onCheckedChange={(v) => setPp(!!v)} />
          <span>
            <a href="/privacy" target="_blank" className="underline">
              プライバシーポリシー
            </a>
            に同意します
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
          <Checkbox checked={sp} onCheckedChange={(v) => setSp(!!v)} />
          <span>
            <a href="/security" target="_blank" className="underline">
              セキュリティポリシー
            </a>
            に同意します
          </span>
        </label>
      </div>
      <div className="pt-2">
        <Button onClick={submit} disabled={!canSubmit || loading}>
          同意して進む
        </Button>
      </div>
    </Card>
  );
}
