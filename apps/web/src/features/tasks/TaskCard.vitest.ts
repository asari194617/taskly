import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { LanguageProvider } from "@/react/LanguageProvider";
import { TaskCard } from "./TaskCard";

const noop = vi.fn();

describe("TaskCard", () => {
  it("shows the deadline period without repeating the grouped date", () => {
    const html = renderToStaticMarkup(
      React.createElement(
        LanguageProvider,
        null,
        React.createElement(TaskCard, {
          t: {
            id: 1,
            title: "プリント",
            subject_id: 10,
            deadline: "2026-10-09",
            deadline_until: "MORNING",
          },
          pendingInfo: false,
          isEditing: false,
          canManage: false,
          subjects: [{ id: 10, name_ja: "数学", name_en: "Math" }],
          editValues: {
            title: "",
            subjectId: "",
            date: "",
            meta: null,
            requireHandIn: false,
            selfCheck: false,
            completionDisabled: false,
            type: "",
            description: "",
            images: [],
          },
          editSetters: {
            title: noop,
            subjectId: noop,
            date: noop,
            meta: noop,
            requireHandIn: noop,
            selfCheck: noop,
            completionDisabled: noop,
            type: noop,
            description: noop,
            images: noop,
          },
          toggleWithPending: noop,
          saveEdit: noop,
          cancelEdit: noop,
          beginEdit: noop,
          onDelete: noop,
          onReport: noop,
        }),
      ),
    );

    expect(html).toContain("数学 朝まで");
    expect(html).not.toContain("2026/10/09");
    expect(html).not.toContain("2026-10-09");
  });
});
