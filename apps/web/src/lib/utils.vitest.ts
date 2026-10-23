import { describe, expect, it } from "vitest";

import { cn } from "./utils";

describe("cn", () => {
  it("keeps the last conflicting Tailwind utility", () => {
    expect(cn("px-2 py-1", "p-3")).toBe("p-3");
  });

  it("combines conditional and variant classes", () => {
    expect(
      cn("bg-background", false && "hidden", {
        "hover:bg-accent": true,
        "text-muted-foreground": false,
      }),
    ).toBe("bg-background hover:bg-accent");
  });

  it("preserves arbitrary values while resolving their conflicts", () => {
    expect(cn("w-[calc(100%-1rem)]", "w-full", "data-open:opacity-100")).toBe(
      "w-full data-open:opacity-100",
    );
  });
});
