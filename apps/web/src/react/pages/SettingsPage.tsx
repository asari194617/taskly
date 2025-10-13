"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { User, Shield, Bell, Key, Calendar, Boxes } from "lucide-react";

export default function SettingsPage() {
  return (
    <SettingsShell>
      <div className="space-y-6">
        <div className="space-y-2">
          <h1 className="text-2xl font-semibold">設定</h1>
          <p className="text-sm text-muted-foreground">
            プロフィール、セキュリティ、通知、APIキー、iCalの設定を行います。
          </p>
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <a href="/settings/basic">
            <Card className="p-4 flex items-start gap-3">
              <User className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">基本設定</div>
                <div className="text-sm text-muted-foreground">
                  表示名・アカウント削除
                </div>
              </div>
            </Card>
          </a>
          <a href="/settings/security">
            <Card className="p-4 flex items-start gap-3">
              <Shield className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">セキュリティ設定</div>
                <div className="text-sm text-muted-foreground">
                  パスキー・TOTP・メールログイン・リカバリーコード
                </div>
              </div>
            </Card>
          </a>
          <a href="/settings/notifications">
            <Card className="p-4 flex items-start gap-3">
              <Bell className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">通知設定</div>
                <div className="text-sm text-muted-foreground">
                  メール・Web Push など（準備中）
                </div>
              </div>
            </Card>
          </a>
          <a href="/settings/api-keys">
            <Card className="p-4 flex items-start gap-3">
              <Key className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">APIキーの設定</div>
                <div className="text-sm text-muted-foreground">
                  APIキーの発行と管理
                </div>
              </div>
            </Card>
          </a>
          <a href="/settings/ical">
            <Card className="p-4 flex items-start gap-3">
              <Calendar className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">iCalカレンダー設定</div>
                <div className="text-sm text-muted-foreground">
                  購読用URLの発行と管理
                </div>
              </div>
            </Card>
          </a>
          <a href="/settings/mcp">
            <Card className="p-4 flex items-start gap-3">
              <Boxes className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">MCP 設定</div>
                <div className="text-sm text-muted-foreground">
                  MCPトークンの発行・無効化、操作履歴
                </div>
              </div>
            </Card>
          </a>
        </div>
      </div>
    </SettingsShell>
  );
}
