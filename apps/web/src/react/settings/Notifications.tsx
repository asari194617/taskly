"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";

export default function SettingsNotifications() {
  const [emailOn, setEmailOn] = React.useState(true);
  const [pushOn, setPushOn] = React.useState(false);
  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">通知設定</h1>
        <Card className="p-4 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <div className="font-medium">メール通知</div>
              <div className="text-sm text-muted-foreground">
                タスクや日直の更新をメールで受け取ります（準備中）。
              </div>
            </div>
            <Switch checked={emailOn} onCheckedChange={setEmailOn} />
          </div>
        </Card>
        <Card className="p-4 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <div className="font-medium">Web Push</div>
              <div className="text-sm text-muted-foreground">
                ブラウザのプッシュ通知（準備中）。
              </div>
            </div>
            <Switch checked={pushOn} onCheckedChange={setPushOn} />
          </div>
        </Card>
      </div>
    </SettingsShell>
  );
}
