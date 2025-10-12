import * as React from "react";
import { Avatar, AvatarFallback, AvatarImage } from "../components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../components/ui/dropdown-menu";
import {
  Home as HomeIcon,
  User as UserIcon,
  LogOut,
  Settings,
} from "lucide-react";
import { getApiBase } from "@/lib/api";
import { toast } from "@/hooks/use-toast";

type Me = { id: number; email: string; display_name?: string | null };

export default function UserMenu() {
  const [me, setMe] = React.useState<Me | null>(null);

  React.useEffect(() => {
    const token =
      (typeof window !== "undefined" && localStorage.getItem("api_token")) ||
      "";
    if (!token) return;
    fetch(`${getApiBase()}/users/me`, { headers: { Authorization: token } })
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (data) setMe(data);
      })
      .catch(() => {
        toast({
          variant: "destructive",
          title: "ユーザー情報の取得に失敗しました",
        });
      });
  }, []);

  const logout = async () => {
    try {
      const token =
        (typeof window !== "undefined" && localStorage.getItem("api_token")) ||
        "";
      if (token) {
        await fetch(`${getApiBase()}/auth/logout`, {
          method: "POST",
          headers: { Authorization: token },
        });
      }
    } catch {}
    try {
      localStorage.removeItem("api_token");
    } catch {}
    toast({ title: "ログアウトしました" });
    window.location.href = "/login";
  };

  const name = me?.display_name || me?.email || "";
  const initials =
    name
      .split(/\s+|@/)
      .filter(Boolean)
      .slice(0, 2)
      .map((s) => s[0]?.toUpperCase())
      .join("") || "U";

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          aria-label="Open user menu"
          className="inline-flex items-center justify-center h-9 w-9 rounded-md border bg-background text-foreground hover:bg-accent hover:text-accent-foreground"
        >
          <Avatar className="h-6 w-6">
            <AvatarImage alt="avatar" />
            <AvatarFallback className="p-0">
              <UserIcon className="h-4 w-4" />
            </AvatarFallback>
          </Avatar>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel className="font-normal">
          <div className="flex flex-col">
            <span className="text-sm font-medium leading-none">
              {name || "ユーザー"}
            </span>
          </div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <a href="/home">
          <DropdownMenuItem className="cursor-pointer">
            <HomeIcon className="mr-2 h-4 w-4" /> ホーム
          </DropdownMenuItem>
        </a>
        <a href="/settings">
          <DropdownMenuItem className="cursor-pointer">
            <Settings className="mr-2 h-4 w-4" /> 設定
          </DropdownMenuItem>
        </a>
        <DropdownMenuItem
          className="text-destructive focus:text-destructive cursor-pointer"
          onClick={logout}
        >
          <LogOut className="mr-2 h-4 w-4" /> ログアウト
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
