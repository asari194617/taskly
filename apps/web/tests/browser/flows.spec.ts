import { expect, test, type Page } from "@playwright/test";

type Task = {
  id: number;
  title: string;
  subject_id: number;
  deadline: string;
  is_completed: boolean;
};

async function mockApi(page: Page, options: { admin?: boolean; failFirstSearch?: boolean; authenticated?: boolean } = {}) {
  await page.addInitScript(() => {
    if (!localStorage.getItem("display_lang")) localStorage.setItem("display_lang", "ja");
  });
  let authenticated = !!options.authenticated;
  let nextId = 2;
  let searchFailures = options.failFirstSearch ? 1 : 0;
  const tasks: Task[] = [];
  const subjects = [{ id: 1, name_ja: "数学", name_en: "Math", created_at: "2026-01-01T00:00:00Z", expires_at: null }];
  await page.route("http://localhost:3063/**", async (route) => {
    const url = new URL(route.request().url());
    const respond = (body: unknown, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (url.pathname === "/auth/options") return respond({ has_password: true, methods: ["password"] });
    if (url.pathname === "/auth/login") {
      const { password } = route.request().postDataJSON();
      if (password !== "browser-password") return respond({ error: "invalid password" }, 401);
      authenticated = true;
      return respond({ authenticated: true });
    }
    if (url.pathname !== "/graphql") return respond({ error: "unexpected API path" }, 404);
    const { query, variables = {} } = route.request().postDataJSON();
    if (query.includes("me{")) {
      if (!authenticated) return respond({ errors: [{ message: "unauthorized" }] }, 401);
      return respond({ data: { me: { id: 1, email: "browser@example.test", display_name: "Browser Tester", is_agreed_policies: true, is_admin: !!options.admin, is_manager: false } } });
    }
    if (query.includes("adminSubjectCreate")) {
      const subject = { id: 2, name_ja: variables.ja, name_en: variables.en || null, created_at: "2026-01-01T00:00:00Z", expires_at: null };
      subjects.push(subject);
      return respond({ data: { adminSubjectCreate: subject } });
    }
    if (query.includes("adminSubjects")) return respond({ data: { adminSubjects: subjects } });
    if (query.includes("subjects{")) return respond({ data: { subjects } });
    if (query.includes("createTask(")) {
      const task = { id: nextId++, title: variables.title, subject_id: variables.subject_id, deadline: variables.deadline, is_completed: false };
      tasks.push(task);
      return respond({ data: { createTask: task } });
    }
    if (query.includes("updateTask(")) {
      const task = tasks.find((item) => item.id === variables.id)!;
      Object.assign(task, { title: variables.title, subject_id: variables.subject_id || task.subject_id });
      return respond({ data: { updateTask: { id: task.id } } });
    }
    if (query.includes("completeTask(")) {
      tasks.find((item) => item.id === variables.id)!.is_completed = true;
      return respond({ data: { completeTask: true } });
    }
    if (query.includes("tasks(")) {
      if (searchFailures-- > 0) return respond({ errors: [{ message: "temporary outage" }] }, 503);
      const matches = tasks.filter((item) => !variables.q || item.title.includes(variables.q));
      return respond({ data: { tasks: { items: matches, next_cursor: null } } });
    }
    throw new Error(`Unmocked GraphQL operation: ${query}`);
  });
  return { tasks, subjects };
}

async function login(page: Page) {
  await page.goto("/login");
  await expect(page.locator('astro-island[component-url*="Begin"]')).not.toHaveAttribute("ssr", "");
  await page.getByLabel("メールアドレス").fill("browser@example.test");
  await page.getByRole("button", { name: "次へ" }).click();
  await expect(page).toHaveURL(/\/login\/password/);
  await expect(page.locator('astro-island[component-url*="Password"]')).not.toHaveAttribute("ssr", "");
  await page.getByLabel("パスワード").fill("browser-password");
  await page.getByRole("button", { name: "次へ" }).click();
  await expect(page).toHaveURL(/\/home$/);
  await expect(page.locator('astro-island[component-url*="TasksList"]')).not.toHaveAttribute("ssr", "");
}

test("ログイン後に課題を作成・編集・完了し、検索できる", async ({ page }) => {
  const api = await mockApi(page);
  await login(page);
  await page.getByRole("button", { name: "新規作成" }).first().click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("タイトル").fill("ブラウザテスト課題");
  await dialog.getByRole("button", { name: "選択してください" }).first().click();
  await page.getByRole("menuitem", { name: "数学" }).click();
  await dialog.getByRole("button", { name: "未設定" }).click();
  await page.getByRole("button", { name: "日付", exact: true }).click();
  await page.locator(".rdp-today button").click();
  await page.getByRole("button", { name: "適用" }).click();
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(page.getByText("ブラウザテスト課題")).toBeVisible();
  await expect.poll(() => api.tasks.length).toBe(1);

  await page.getByRole("button", { name: "アクション" }).click();
  await page.getByRole("button", { name: "編集" }).click();
  await page.getByLabel("タイトル").last().fill("編集済み課題");
  await page.getByRole("button", { name: "保存" }).click();
  await expect(page.getByText("編集済み課題")).toBeVisible();
  await expect.poll(() => api.tasks[0].title).toBe("編集済み課題");

  await page.getByRole("radio", { name: "完了", exact: true }).click();
  await expect.poll(() => api.tasks[0].is_completed, { timeout: 6_000 }).toBe(true);
  await page.getByPlaceholder("課題を検索 (タイトル/説明)").fill("編集済み");
  await expect(page.getByText("編集済み課題")).toBeVisible();
  await page.getByPlaceholder("課題を検索 (タイトル/説明)").fill("存在しない課題");
  await expect(page.getByText("検索結果がありません")).toBeVisible();
});

test("課題の完了フィルターをキーボードで切り替え・解除できる", async ({ page }) => {
  const api = await mockApi(page);
  api.tasks.push(
    { id: 1, title: "未完了の課題", subject_id: 1, deadline: "2026-09-23", is_completed: false },
    { id: 2, title: "完了した課題", subject_id: 1, deadline: "2026-09-23", is_completed: true },
  );
  await login(page);

  const incomplete = page.getByRole("button", { name: "未完了", exact: true });
  const completed = page.getByRole("button", { name: "完了済み", exact: true });
  await expect(incomplete).toHaveAttribute("aria-pressed", "false");
  await expect(completed).toHaveAttribute("aria-pressed", "false");

  await page.getByPlaceholder("課題を検索 (タイトル/説明)").focus();
  await page.keyboard.press("Tab");
  await expect(page.getByRole("button", { name: "新規作成" }).first()).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(incomplete).toBeFocused();
  expect(await incomplete.evaluate((element) => getComputedStyle(element).boxShadow)).not.toBe("none");
  await page.keyboard.press("Enter");
  await expect(incomplete).toHaveAttribute("aria-pressed", "true");
  await expect(completed).toHaveAttribute("aria-pressed", "false");
  await expect(page.getByText("未完了の課題")).toBeVisible();
  await expect(page.getByText("完了した課題")).toHaveCount(0);

  await page.keyboard.press("Space");
  await expect(incomplete).toHaveAttribute("aria-pressed", "false");
  await expect(page.getByText("完了した課題")).toBeVisible();

  await page.keyboard.press("Tab");
  await expect(completed).toBeFocused();
  await page.keyboard.press("Space");
  await expect(completed).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText("未完了の課題")).toHaveCount(0);
  await page.keyboard.press("Enter");
  await expect(completed).toHaveAttribute("aria-pressed", "false");
  await expect(page.getByText("未完了の課題")).toBeVisible();

  await completed.click();
  await expect(completed).toHaveAttribute("aria-pressed", "true");
  await completed.click();
  await expect(completed).toHaveAttribute("aria-pressed", "false");
});

test("モバイルの新規作成ボタンを名前で見つけて作成画面を開ける", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockApi(page);
  await login(page);

  const createButton = page.getByRole("button", { name: "新規作成", exact: true })
    .and(page.locator('button[aria-label="新規作成"]'));
  await expect(createButton).toBeVisible();
  await createButton.click();
  await expect(page.getByRole("dialog").getByText("新規課題の作成")).toBeVisible();
});

test("API 失敗を表示し、再読み込みで復帰する", async ({ page }) => {
  await mockApi(page, { failFirstSearch: true });
  await login(page);
  await expect(page.getByText("temporary outage")).toBeVisible();
  await page.getByRole("button", { name: "再読み込み" }).click();
  await expect(page.getByRole("button", { name: "新規作成" }).first()).toBeVisible();
});

test("管理者が教科を作成できる", async ({ page }) => {
  const api = await mockApi(page, { admin: true });
  await login(page);
  await page.goto("/admin/subject");
  await expect(page.locator('astro-island[component-url*="Subjects"]')).not.toHaveAttribute("ssr", "");
  await expect(page.getByRole("heading", { name: "教科管理" })).toBeVisible();
  await page.getByRole("button", { name: "教科を追加" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByPlaceholder("例: 数学").fill("理科");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(page.getByRole("cell", { name: "理科" })).toBeVisible();
  expect(api.subjects.at(-1)?.name_ja).toBe("理科");
});

test("通知スイッチの名前・状態が日本語と英語で読み取れ、キーボードで操作できる", async ({ page }) => {
  await mockApi(page);
  await page.route("http://localhost:3063/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/settings/notification-methods") {
      return route.fulfill({ json: { email: true, webpush: false } });
    }
    if (path === "/settings/notification-types") {
      return route.fulfill({ json: { TASK: true, TASK_MORNING: true, TASK_EVENING: false, DAYSHIFT: true } });
    }
    if (path === "/push/devices") return route.fulfill({ json: [] });
    if (path === "/push/vapidPublicKey") return route.fulfill({ json: { publicKey: "unused" } });
    return route.fallback();
  });
  await login(page);
  await page.goto("/settings/notifications");
  await expect(page.locator('astro-island[component-url*="Notifications"]')).not.toHaveAttribute("ssr", "");
  await page.getByRole("button", { name: "通知方法" }).click();
  await page.getByRole("button", { name: "通知の種類" }).click();

  for (const [name, checked] of [
    ["メール", true], ["Webプッシュ通知", false], ["期限が近い課題", true],
    ["朝の通知", true], ["夕方の通知", false], ["日直のお知らせ", true],
  ] as const) {
    await expect(page.getByRole("switch", { name, exact: true })).toHaveAttribute("aria-checked", String(checked));
  }

  const email = page.getByRole("switch", { name: "メール", exact: true });
  await email.focus();
  await page.keyboard.press("Space");
  await expect(email).toHaveAttribute("aria-checked", "false");
  await page.keyboard.press("Tab");
  const push = page.getByRole("switch", { name: "Webプッシュ通知" });
  if (await push.isEnabled()) {
    await expect(push).toBeFocused();
  } else {
    await expect(page.getByRole("button", { name: "通知の種類" })).toBeFocused();
  }

  await page.evaluate(() => {
    localStorage.setItem("display_lang", "en");
    window.dispatchEvent(new Event("languagechange"));
  });
  for (const name of ["Email", "Web push notifications", "Tasks due soon", "Morning notifications", "Evening notifications", "Day shift notifications"]) {
    await expect(page.getByRole("switch", { name, exact: true })).toBeVisible();
  }
  await expect(page.getByRole("switch", { name: "Email", exact: true })).toHaveAttribute("aria-checked", "false");
});

test("通知設定と同意画面が表示言語の切替に追従する", async ({ page }) => {
  await mockApi(page);
  await login(page);
  await page.goto("/settings/notifications");
  await expect(page.getByRole("heading", { name: "通知設定" }).first()).toBeVisible();
  await expect(page.locator('astro-island[component-url*="AppShell"]:not([ssr])')).toHaveCount(1);
  await page.getByRole("combobox", { name: "表示言語" }).click();
  await page.getByRole("option", { name: "English" }).click();
  await expect(page.getByRole("heading", { name: "Notification Settings" }).first()).toBeVisible();
  await page.getByRole("button", { name: "Notification methods" }).click();
  await expect(page.getByText("Web push notifications", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Notification types" }).click();
  await expect(page.getByText("Tasks due soon", { exact: true })).toBeVisible();
  await expect(page.getByText("To receive notifications, enable both a notification method and a notification type.")).toBeVisible();

  await page.goto("/consent");
  await expect(page.getByRole("heading", { name: "Before you continue" })).toBeVisible();
  await expect(page.getByText("I agree to the Terms")).toBeVisible();
  await expect(page.locator('astro-island[component-url*="AppShell"]:not([ssr])')).toHaveCount(1);
  await page.getByRole("combobox", { name: "Language" }).click();
  await page.getByRole("option", { name: "Japanese" }).click();
  await expect(page.getByRole("heading", { name: "ご利用にあたって" })).toBeVisible();
});

test("設定詳細画面と日時選択の文言が英語・日本語に切り替わる", async ({ page }) => {
  test.setTimeout(60_000);
  await mockApi(page, { authenticated: true });
  await page.route("http://localhost:3063/graphql", async (route) => {
    const query = route.request().postDataJSON()?.query || "";
    const respond = (data: unknown) => route.fulfill({ json: { data } });
    if (query.includes("apiKeys{")) return respond({ apiKeys: [
      { id: 1, description: "CI", is_valid: true, created_at: "2026-01-01T00:00:00Z", expires_at: "2027-01-01T00:00:00Z" },
    ] });
    if (query.includes("icalTokens{")) return respond({ icalTokens: [
      { id: 2, description: "Personal", url: "https://example.test/calendar", created_at: "2026-01-01T00:00:00Z", expires_at: "2027-01-01T00:00:00Z" },
    ] });
    if (query.includes("mcpInfo{")) return respond({ mcpInfo: { enabled: true, has_token: true, expires_at: "2027-01-01T00:00:00Z" } });
    if (query.includes("myLogs(")) return respond({ myLogs: [] });
    return route.fallback();
  });
  await page.goto("/settings/basic");
  await expect(page.locator('astro-island[component-url*="AppShell"]')).not.toHaveAttribute("ssr", "");
  await page.getByRole("combobox", { name: "表示言語" }).click();
  await page.getByRole("option", { name: "English" }).click();
  await expect(page.getByRole("heading", { name: "Basic Settings" }).first()).toBeVisible();
  await expect(page.getByText("Email language")).toBeVisible();

  await page.goto("/settings/api-keys");
  await expect(page.getByRole("heading", { name: "API Keys" }).first()).toBeVisible();
  await expect(page.getByRole("columnheader", { name: "Expiration" })).toBeVisible();
  await page.getByRole("button", { name: "Update expiration" }).click();
  await expect(page.getByRole("dialog").getByText("Set or update expiration")).toBeVisible();
  await page.getByRole("dialog").getByRole("button", { name: /2027\/01\/01/ }).click();
  await expect(page.getByText("Time", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Clear" })).toBeVisible();

  await page.goto("/settings/ical");
  await expect(page.getByRole("button", { name: "Update expiration" })).toBeVisible();
  await page.getByRole("button", { name: "Delete" }).last().click();
  await expect(page.getByRole("dialog").getByText("Delete this iCal URL?")).toBeVisible();

  await page.goto("/settings/mcp");
  await expect(page.getByRole("heading", { name: "MCP Settings" }).first()).toBeVisible();
  await page.getByRole("button", { name: "Update expiration" }).click();
  await expect(page.getByRole("dialog").getByText("Expiration (optional)")).toBeVisible();
  await page.evaluate(() => {
    localStorage.setItem("display_lang", "ja");
    window.dispatchEvent(new Event("languagechange"));
  });
  await expect(page.getByRole("dialog").getByText("有効期限(任意)")).toBeVisible();
});
