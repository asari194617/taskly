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
  LogIn,
  LogOut,
  Settings,
} from "lucide-react";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import { t, type Lang } from "@/lib/i18n";

type Me = { id: number; email: string; display_name?: string | null };

export default function UserMenu() {
  const [me, setMe] = React.useState<Me | null>(null);
  const [authed, setAuthed] = React.useState(false);
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
    gql<{ me: Me }>(`query{ me{ id email display_name } }`, undefined, {
      redirectOnUnauthorized: false,
    })
      .then((data) => {
        setMe(data.me);
        setAuthed(true);
      })
      .catch(() => {
        setAuthed(false);
      });
  }, []);

  const logout = async () => {
    try {
      const result = await gql<{ logout: boolean }>(`mutation{ logout }`);
      if (!result.logout) throw new Error("logout failed");
      toast({ title: dict.toast.loggedOut });
      window.location.href = "/login";
    } catch {
      toast({
        variant: "destructive",
        title: lang === "ja" ? "ログアウトに失敗しました" : "Logout failed",
      });
    }
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
        {authed ? (
          <>
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
          </>
        ) : (
          <a href="/login">
            <DropdownMenuItem className="cursor-pointer">
              <LogIn className="mr-2 h-4 w-4" /> {dict.common.login}
            </DropdownMenuItem>
          </a>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
