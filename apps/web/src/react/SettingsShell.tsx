"use client";
import * as React from "react";
import { createPortal } from "react-dom";
import {
  SidebarProvider,
  SidebarTrigger,
  SidebarInset,
} from "@/components/ui/sidebar";
import { AppSidebar } from "@/components/app-sidebar";
import { useLang } from "./LanguageProvider";
import { Menu } from "lucide-react";

export default function SettingsShell({
  children,
}: {
  children: React.ReactNode;
}) {
  const { dict } = useLang();
  const [headerActions, setHeaderActions] = React.useState<HTMLElement | null>(
    null
  );

  React.useEffect(() => {
    setHeaderActions(document.getElementById("header-start-actions"));
  }, []);

  return (
    <SidebarProvider className="relative">
      <AppSidebar />
      <SidebarInset>
        {headerActions &&
          createPortal(
            <SidebarTrigger
              aria-label={dict.settings.openMenu}
              title={dict.settings.openMenu}
              className="h-10 w-auto gap-2 border bg-background px-3 shadow-sm hover:bg-muted md:hidden"
            >
              <Menu className="size-5" aria-hidden="true" />
              <span className="text-sm font-medium">{dict.common.menu}</span>
            </SidebarTrigger>,
            headerActions
          )}
        <div className="hidden h-12 items-center gap-2 border-b px-3 md:flex">
          <SidebarTrigger aria-label={dict.settings.toggleMenu} />
          <div className="text-sm text-muted-foreground">
            {dict.settings.title}
          </div>
        </div>
        <div className="p-4 sm:p-6 max-w-3xl">{children}</div>
      </SidebarInset>
    </SidebarProvider>
  );
}
