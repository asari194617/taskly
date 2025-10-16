"use client";
import * as React from "react";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar";
import {
  User,
  Shield,
  Bell,
  Key,
  Calendar,
  Settings,
  Boxes,
} from "lucide-react";
import { useLang } from "@/react/LanguageProvider";

type MenuItem = {
  href: string;
  label: string;
  icon: React.ComponentType<React.SVGProps<SVGSVGElement>>;
};

function useItems(dict: ReturnType<typeof import("@/lib/i18n").t>) {
  const d = dict.settings;
  const items: MenuItem[] = [
    { href: "/settings/basic", label: d.basic, icon: User },
    { href: "/settings/security", label: d.security, icon: Shield },
    { href: "/settings/notifications", label: d.notifications, icon: Bell },
    { href: "/settings/api-keys", label: d.apiKeys, icon: Key },
    { href: "/settings/ical", label: d.ical, icon: Calendar },
    { href: "/settings/mcp", label: d.mcp, icon: Boxes },
  ];
  return items;
}

export function AppSidebar() {
  const [path, setPath] = React.useState<string>("/settings");
  const { dict } = useLang();
  const items = useItems(dict);
  React.useEffect(() => {
    if (typeof window !== "undefined") {
      setPath(window.location.pathname);
    }
  }, []);

  return (
    <Sidebar collapsible="offcanvas">
      <SidebarHeader>
        <div className="px-2 py-1 text-sm font-medium inline-flex items-center gap-2">
          <Settings className="h-4 w-4" /> {dict.settings.title}
        </div>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>{dict.common.menu}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {items.map((item) => (
                <SidebarMenuItem key={item.href}>
                  <SidebarMenuButton
                    asChild
                    isActive={path.startsWith(item.href)}
                  >
                    <a href={item.href} className="flex items-center gap-2">
                      <item.icon className="h-4 w-4" />
                      <span>{item.label}</span>
                    </a>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <div className="px-3 py-2 text-xs text-muted-foreground">Taskly</div>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}

export default AppSidebar;
