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
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import { t, type Lang } from "@/lib/i18n";

type Me = { id: number; email: string; display_name?: string | null };

export default function UserMenu() {
  const [me, setMe] = React.useState<Me | null>(null);
  const [lang, setLang] = React.useState<Lang>("ja");

  // 言語状態を管理
  React.useEffect(() => {
    const updateLang = () => {
      const stored = localStorage.getItem("display_lang") as Lang | null;
      setLang(stored || "ja");
    };

    updateLang();

    // 言語変更イベントを監視
    window.addEventListener("languagechange", updateLang);
    window.addEventListener("storage", (e) => {
      if (e.key === "display_lang") {
        updateLang();
      }
    });

    return () => {
      window.removeEventListener("languagechange", updateLang);
    };
  }, []);

  const dict = React.useMemo(() => t(lang), [lang]);

  React.useEffect(() => {
    const token =
      (typeof window !== "undefined" && localStorage.getItem("api_token")) ||
      "";
    if (!token) return;
    gql<{ me: Me }>(`query{ me{ id email display_name } }`)
      .then((data) => setMe(data.me))
      .catch(() =>
        toast({
          variant: "destructive",
          title: dict.toast.userInfoFetchError,
        })
      );
  }, [dict.toast.userInfoFetchError]);

  const logout = async () => {
    try {
      await gql<{ logout: boolean }>(`mutation{ logout }`);
    } catch {}
    try {
      localStorage.removeItem("api_token");
    } catch {}
    toast({ title: dict.toast.loggedOut });
    window.location.href = "/login";
  };

  const rawName = me?.display_name || me?.email || "";
  const name = lang === "ja" && rawName ? `${rawName} さん` : rawName;
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
              {name || dict.common.user}
            </span>
          </div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <a href="/home">
          <DropdownMenuItem className="cursor-pointer">
            <HomeIcon className="mr-2 h-4 w-4" /> {dict.common.home}
          </DropdownMenuItem>
        </a>
        <a href="/settings">
          <DropdownMenuItem className="cursor-pointer">
            <Settings className="mr-2 h-4 w-4" /> {dict.common.settings}
          </DropdownMenuItem>
        </a>
        <DropdownMenuItem
          className="text-destructive focus:text-destructive cursor-pointer"
          onClick={logout}
        >
          <LogOut className="mr-2 h-4 w-4" /> {dict.common.logout}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
