"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { User, Shield, Bell, Key, Calendar, Boxes } from "lucide-react";
import { t, type Lang } from "@/lib/i18n";
import { useLang } from "@/react/LanguageProvider";

export default function SettingsPage() {
  const { dict } = useLang();

  return (
    <SettingsShell>
      <div className="space-y-6">
        <div className="space-y-2">
          <h1 className="text-2xl font-semibold">{dict.settings.title}</h1>
          <p className="text-sm text-muted-foreground">
            {dict.settings.description}
          </p>
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <a href="/settings/basic">
            <Card className="p-4 flex items-start gap-3">
              <User className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">{dict.settings.basic}</div>
                <div className="text-sm text-muted-foreground">
                  {dict.settings.basicDesc}
                </div>
              </div>
            </Card>
          </a>
          <a href="/settings/security">
            <Card className="p-4 flex items-start gap-3">
              <Shield className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">{dict.settings.security}</div>
                <div className="text-sm text-muted-foreground">
                  {dict.settings.securityDesc}
                </div>
              </div>
            </Card>
          </a>
          <a href="/settings/notifications">
            <Card className="p-4 flex items-start gap-3">
              <Bell className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">{dict.settings.notifications}</div>
                <div className="text-sm text-muted-foreground">
                  {dict.settings.notificationsDesc}
                </div>
              </div>
            </Card>
          </a>
          <a href="/settings/api-keys">
            <Card className="p-4 flex items-start gap-3">
              <Key className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">{dict.settings.apiKeys}</div>
                <div className="text-sm text-muted-foreground">
                  {dict.settings.apiKeysDesc}
                </div>
              </div>
            </Card>
          </a>
          <a href="/settings/ical">
            <Card className="p-4 flex items-start gap-3">
              <Calendar className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">{dict.settings.ical}</div>
                <div className="text-sm text-muted-foreground">
                  {dict.settings.icalDesc}
                </div>
              </div>
            </Card>
          </a>
          <a href="/settings/mcp">
            <Card className="p-4 flex items-start gap-3">
              <Boxes className="h-5 w-5 mt-0.5" />
              <div>
                <div className="font-medium">{dict.settings.mcp}</div>
                <div className="text-sm text-muted-foreground">
                  {dict.settings.mcpDesc}
                </div>
              </div>
            </Card>
          </a>
        </div>
      </div>
    </SettingsShell>
  );
}
