import { expect, test, type Page } from "@playwright/test";

test.use({ serviceWorkers: "block" });
async function mockCompletionAPI(page: Page) {
  const state = {
    tasks: [
      {
        id: 1,
        title: "橋渡し課題A",
        subject_id: 1,
        deadline: "2099-01-01",
        is_completed: false,
      },
      {
        id: 2,
        title: "橋渡し課題B",
        subject_id: 1,
        deadline: "2099-01-01",
        is_completed: false,
      },
    ],
    reads: 0,
    writes: 0,
    queries: [] as string[],
    holdNextRead: false,
    holdWrite: false,
    failNextRead: false,
    releaseRead: undefined as (() => void) | undefined,
    releaseWrite: undefined as (() => void) | undefined,
  };
  await page.addInitScript(() => localStorage.setItem("display_lang", "ja"));
  await page.route("http://localhost:3063/**", async (route) => {
    const headers = {
      "Access-Control-Allow-Origin": `http://localhost:${process.env.PLAYWRIGHT_PORT || "4321"}`,
      "Access-Control-Allow-Credentials": "true",
      "Access-Control-Allow-Headers": "Content-Type, X-Client-Platform",
      "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
    };
    if (route.request().method() === "OPTIONS")
      return route.fulfill({ status: 204, headers });
    const { query, variables = {} } = route.request().postDataJSON();
    const respond = (data: unknown, status = 200) =>
      route.fulfill({
        status,
        headers,
        contentType: "application/json",
        body: JSON.stringify(data),
      });
    if (query.includes("me{"))
      return respond({
        data: {
          me: {
            id: 1,
            email: "sync@example.test",
            display_name: "Sync Test",
            is_agreed_policies: true,
            is_admin: false,
            is_manager: false,
          },
        },
      });
    if (query.includes("subjects{"))
      return respond({
        data: { subjects: [{ id: 1, name_ja: "検証科目", name_en: "Test" }] },
      });
    if (query.includes("taskCompletionDefaults"))
      return respond({ data: { taskCompletionDefaults: [] } });
    if (query.includes("tasks(")) {
      state.reads++;
      state.queries.push(variables.q || "");
      if (state.failNextRead) {
        state.failNextRead = false;
        return respond({ errors: [{ message: "temporary offline" }] }, 503);
      }
      const snapshot = structuredClone(
        state.tasks.filter(
          (task) => !variables.q || task.title.includes(variables.q),
        ),
      );
      if (state.holdNextRead) {
        state.holdNextRead = false;
        await new Promise<void>((resolve) => {
          state.releaseRead = resolve;
        });
      }
      try {
        return await respond({
          data: { tasks: { items: snapshot, next_cursor: null } },
        });
      } catch {
        return;
      }
    }
    if (query.includes("completeTask(")) {
      state.writes++;
      if (state.holdWrite)
        await new Promise<void>((resolve) => {
          state.releaseWrite = resolve;
        });
      state.tasks.find((task) => task.id === variables.id)!.is_completed =
        !query.includes("uncompleteTask(");
      return respond({
        data: query.includes("uncompleteTask(")
          ? { uncompleteTask: true }
          : { completeTask: true },
      });
    }
    throw new Error(`Unexpected mock operation ${query}`);
  });
  await page.clock.install();
  await page.goto("/home");
  await expect(
    page.getByRole("checkbox", { name: "橋渡し課題A: 完了", exact: true }),
  ).toBeVisible();
  return state;
}
const focus = (page: Page) =>
  page.evaluate(() => window.dispatchEvent(new Event("focus")));

test("他サービスの完了変更をフォーカスと30秒更新で反映し、検索・絞り込み・入力中フォームを維持する", async ({
  page,
}) => {
  const api = await mockCompletionAPI(page);
  const search = page.getByPlaceholder("課題を検索 (タイトル/説明)");
  await search.fill("橋渡し");
  await page.clock.fastForward(501);
  await expect.poll(() => api.queries.at(-1)).toBe("橋渡し");
  const incomplete = page.getByRole("button", { name: "未完了", exact: true });
  await incomplete.click();
  api.tasks[0].is_completed = true;
  await focus(page);
  await expect(
    page.getByRole("checkbox", { name: "橋渡し課題A: 完了", exact: true }),
  ).toHaveCount(0);
  await expect(search).toHaveValue("橋渡し");
  await expect(incomplete).toHaveAttribute("aria-pressed", "true");
  api.tasks[0].is_completed = false;
  await page.clock.fastForward(30_000);
  await expect(
    page.getByRole("checkbox", { name: "橋渡し課題A: 完了", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "新規作成", exact: true })
    .first()
    .click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("タイトル").fill("保存前の入力");
  api.tasks[1].is_completed = true;
  await focus(page);
  await expect(dialog.getByLabel("タイトル")).toHaveValue("保存前の入力");
  await page.keyboard.press("Escape");
  await expect(dialog).not.toBeVisible();
  await expect(
    page.getByRole("checkbox", { name: "橋渡し課題B: 完了", exact: true }),
  ).toHaveCount(0);
  const beforeFailure = api.reads;
  api.failNextRead = true;
  await focus(page);
  await expect.poll(() => api.reads).toBe(beforeFailure + 1);
  await expect(search).toHaveValue("橋渡し");
  await expect(
    page.getByRole("checkbox", { name: "橋渡し課題A: 完了", exact: true }),
  ).toBeVisible();
  const beforeHidden = api.reads;
  await page.evaluate(() => {
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "hidden",
    });
    document.dispatchEvent(new Event("visibilitychange"));
  });
  api.tasks[0].is_completed = true;
  await focus(page);
  await page.clock.fastForward(60_000);
  expect(api.reads).toBe(beforeHidden);
  await page.evaluate(() => {
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "visible",
    });
    document.dispatchEvent(new Event("visibilitychange"));
  });
  await expect(
    page.getByRole("checkbox", { name: "橋渡し課題A: 完了", exact: true }),
  ).toHaveCount(0);
  await expect(incomplete).toHaveAttribute("aria-pressed", "true");
});

test("取り消し期間と送信待ちの完了状態を古い一覧取得で巻き戻さない", async ({
  page,
}) => {
  const api = await mockCompletionAPI(page),
    checkbox = page.getByRole("checkbox", {
      name: "橋渡し課題A: 完了",
      exact: true,
    });
  await checkbox.click();
  await expect(checkbox).toBeChecked();
  const beforeUndo = api.reads;
  await focus(page);
  await page.clock.fastForward(1_000);
  expect(api.reads).toBe(beforeUndo);
  await checkbox.click();
  await expect(checkbox).not.toBeChecked();
  await expect.poll(() => api.reads).toBeGreaterThan(beforeUndo);
  expect(api.writes).toBe(0);
  api.holdNextRead = true;
  const beforeStale = api.reads;
  await focus(page);
  await expect.poll(() => api.reads).toBe(beforeStale + 1);
  api.holdWrite = true;
  await checkbox.click();
  await expect(checkbox).toBeChecked();
  await page.clock.fastForward(3_000);
  await expect.poll(() => api.writes).toBe(1);
  const whileWriting = api.reads;
  await focus(page);
  await page.clock.fastForward(30_000);
  expect(api.reads).toBe(whileWriting);
  await expect(checkbox).toBeChecked();
  api.releaseWrite!();
  await expect.poll(() => api.reads).toBeGreaterThan(whileWriting);
  await expect(checkbox).toBeChecked();
  api.releaseRead!();
  await page.clock.fastForward(1);
  await expect(checkbox).toBeChecked();
});
