"use client";
import * as React from "react";
import {
  SidebarProvider,
  SidebarTrigger,
  SidebarInset,
} from "@/components/ui/sidebar";
import { AppSidebar } from "@/components/app-sidebar";
import { useLang } from "./LanguageProvider";

export default function SettingsShell({
  children,
}: {
  children: React.ReactNode;
}) {
  const { dict } = useLang();
  return (
    <SidebarProvider className="relative">
      <AppSidebar />
      <SidebarInset>
        <div className="flex h-12 items-center gap-2 border-b px-3">
          <SidebarTrigger />
          <div className="text-sm text-muted-foreground">
            {dict.settings.title}
          </div>
        </div>
        <div className="p-4 sm:p-6 max-w-3xl">{children}</div>
      </SidebarInset>
    </SidebarProvider>
  );
}
