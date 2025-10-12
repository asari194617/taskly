"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

export default function SettingsBasic() {
  const [displayName, setDisplayName] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";

  React.useEffect(() => {
    async function fetchMe() {
      try {
        const res = await fetch("/users/me", {
          headers: { Authorization: token },
        });
        if (res.ok) {
          const data = await res.json();
          setDisplayName(data.display_name || "");
        }
      } catch {}
    }
    if (token) fetchMe();
  }, [token]);

  async function save() {
    setLoading(true);
    try {
      const res = await fetch("/users/me", {
        method: "PATCH",
        headers: { "Content-Type": "application/json", Authorization: token },
        body: JSON.stringify({ display_name: displayName }),
      });
      if (!res.ok) alert("保存に失敗しました");
    } finally {
      setLoading(false);
    }
  }

  async function deleteAccount() {
    if (!confirm("本当にアカウントを削除しますか？")) return;
    const res = await fetch("/users/me", {
      method: "DELETE",
      headers: { Authorization: token },
    });
    if (res.ok) {
      localStorage.removeItem("api_token");
      location.href = "/";
    } else {
      alert("削除に失敗しました");
    }
  }

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">基本設定</h1>
        <Card className="p-4 space-y-3">
          <div className="space-y-1">
            <Label htmlFor="display_name">表示名</Label>
            <Input
              id="display_name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
            />
          </div>
          <Button onClick={save} disabled={loading}>
            {loading ? "保存中..." : "保存"}
          </Button>
        </Card>
        <Card className="p-4 space-y-3 border-destructive/30">
          <div>
            <div className="font-medium text-destructive">アカウント削除</div>
            <div className="text-sm text-muted-foreground">
              この操作は取り消せません。
            </div>
          </div>
          <Dialog>
            <DialogTrigger asChild>
              <Button variant="destructive">アカウントを削除</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>本当に削除しますか？</DialogTitle>
              </DialogHeader>
              <DialogFooter>
                <Button variant="secondary">キャンセル</Button>
                <Button variant="destructive" onClick={deleteAccount}>
                  削除する
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </Card>
      </div>
    </SettingsShell>
  );
}
