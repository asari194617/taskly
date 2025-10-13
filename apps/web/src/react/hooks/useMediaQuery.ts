"use client";
import * as React from "react";

export function useMediaQuery(query: string) {
  const [matches, setMatches] = React.useState<boolean>(() => {
    if (typeof window === "undefined") return true;
    return window.matchMedia(query).matches;
  });
  React.useEffect(() => {
    if (typeof window === "undefined") return;
    const m = window.matchMedia(query);
    const handler = () => setMatches(m.matches);
    if (typeof m.addEventListener === "function")
      m.addEventListener("change", handler);
    else (m as any).addListener?.(handler);
    handler();
    return () => {
      if (typeof m.removeEventListener === "function")
        m.removeEventListener("change", handler);
      else (m as any).removeListener?.(handler);
    };
  }, [query]);
  return matches;
}
