import { expect, test, type Page } from "@playwright/test";

// Mocked API requests must stay visible to Playwright, including in WebKit.
test.use({ serviceWorkers: "block" });

const corsHeaders = {
  "Access-Control-Allow-Origin": `http://localhost:${process.env.PLAYWRIGHT_PORT || "4321"}`,
  "Access-Control-Allow-Credentials": "true",
  "Access-Control-Allow-Headers": "Content-Type, X-Client-Platform, X-Amz-Meta-Owner-Id",
  "Access-Control-Allow-Methods": "GET, POST, PUT, OPTIONS",
};

type Task = {
  id: number;
  title: string;
  subject_id: number;
  deadline: string;
  description?: string;
  image_keys?: string[];
  image_urls?: string[];
  is_completed: boolean;
  is_completion_disabled?: boolean;
  type?: string;
};

async function mockApi(page: Page, options: { admin?: boolean; failFirstSearch?: boolean; authenticated?: boolean; meDelayMs?: number } = {}) {
  await page.addInitScript(() => {
    if (!localStorage.getItem("display_lang")) localStorage.setItem("display_lang", "ja");
  });
  let authenticated = !!options.authenticated;
  let nextId = 2;
  let searchFailures = options.failFirstSearch ? 1 : 0;
  const tasks: Task[] = [];
  const completionDefaults = Object.fromEntries(["ONLINE", "FACE_TO_FACE", "QUIZ", "PRESENTATION", "SELF_STUDY", "OTHER"].map((type) => [type, type === "QUIZ"]));
  const uploadedKeys: string[] = [];
  let presignCount = 0;
  const subjects = [{ id: 1, name_ja: "数学", name_en: "Math", created_at: "2026-01-01T00:00:00Z", expires_at: null }];
  await page.route("https://r2.example.test/**", async (route) => {
    if (route.request().method() === "PUT") uploadedKeys.push(route.request().url());
    return route.fulfill({ status: 200, headers: corsHeaders, body: "" });
  });
  await page.route("http://localhost:3063/**", async (route) => {
    const url = new URL(route.request().url());
    const respond = (body: unknown, status = 200) => route.fulfill({ status, headers: corsHeaders, contentType: "application/json", body: JSON.stringify(body) });
    if (route.request().method() === "OPTIONS") return route.fulfill({ status: 204, headers: corsHeaders });
    if (url.pathname === "/auth/oauth/start") {
      authenticated = true;
      return route.fulfill({ status: 302, headers: { Location: `http://localhost:${process.env.PLAYWRIGHT_PORT || "4321"}/home` }, body: "" });
    }
    if (url.pathname === "/task-images/presign") {
      presignCount += 1;
      const suffix = String(presignCount).padStart(12, "0");
      return respond({
        key: `task-images/1/550e8400-e29b-41d4-a716-${suffix}`,
        upload_url: `https://r2.example.test/signed-upload-${presignCount}`,
        headers: { "Content-Type": "image/png", "X-Amz-Meta-Owner-Id": "1" },
      });
    }
    if (url.pathname !== "/graphql") return respond({ error: "unexpected API path" }, 404);
    const { query, variables = {} } = route.request().postDataJSON();
    if (query.includes("me{")) {
      if (options.meDelayMs) {
        await new Promise((resolve) => setTimeout(resolve, options.meDelayMs));
      }
      if (!authenticated) return respond({ errors: [{ message: "unauthorized" }] }, 401);
      return respond({ data: { me: { id: 1, email: "browser@example.test", display_name: "Browser Tester", is_agreed_policies: true, is_admin: !!options.admin, is_manager: false } } });
    }
    if (query.includes("taskCompletionDefaults")) return respond({ data: { taskCompletionDefaults: Object.entries(completionDefaults).map(([type, disabled]) => ({ type, is_completion_disabled: disabled })) } });
    if (query.includes("adminSetTaskCompletionDefault")) {
      completionDefaults[variables.type] = variables.disabled;
      return respond({ data: { adminSetTaskCompletionDefault: true } });
    }
    if (query.includes("adminSubjectCreate")) {
      const subject = { id: 2, name_ja: variables.ja, name_en: variables.en || null, created_at: "2026-01-01T00:00:00Z", expires_at: null };
      subjects.push(subject);
      return respond({ data: { adminSubjectCreate: subject } });
    }
    if (query.includes("adminSubjects")) return respond({ data: { adminSubjects: subjects } });
    if (query.includes("subjects{")) return respond({ data: { subjects } });
    if (query.includes("createTask(")) {
      const task = { id: nextId++, title: variables.title, subject_id: variables.subject_id, deadline: variables.deadline, description: variables.description, type: variables.type || "OTHER", is_completion_disabled: variables.completionDisabled ?? completionDefaults[variables.type || "OTHER"], image_keys: variables.image_keys || [], image_urls: (variables.image_keys || []).map((key: string) => `https://r2.example.test/view/${key}`), is_completed: false };
      tasks.push(task);
      return respond({ data: { createTask: task } });
    }
    if (query.includes("updateTask(")) {
      const task = tasks.find((item) => item.id === variables.id)!;
      Object.assign(task, { is_completion_disabled: variables.completionDisabled ?? task.is_completion_disabled, title: variables.title, subject_id: variables.subject_id || task.subject_id, type: variables.type || task.type, image_keys: variables.image_keys ?? task.image_keys, image_urls: (variables.image_keys ?? task.image_keys ?? []).map((key: string) => `https://r2.example.test/view/${key}`) });
      if (task.is_completion_disabled) task.is_completed = false;
      return respond({ data: { updateTask: task } });
    }
    if (query.includes("uncompleteTask(")) {
      tasks.find((item) => item.id === variables.id)!.is_completed = false;
      return respond({ data: { uncompleteTask: true } });
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
  return { tasks, subjects, uploadedKeys };
}

async function login(page: Page) {
  await page.goto("/login");
  await expect(page).toHaveURL(/\/home$/);
  await expect(page.locator('astro-island[component-url*="TasksList"]')).not.toHaveAttribute("ssr", "");
}

test("ログイン後に課題を作成・編集・完了し、検索できる", async ({ page }) => {
  const api = await mockApi(page);
  await login(page);
  await page.getByRole("button", { name: "新規作成" }).first().click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("タイトル").fill("ブラウザテスト課題");
  await dialog.getByLabel("サブジェクト", { exact: true }).click();
  await page.getByRole("menuitem", { name: "数学" }).click();
  await dialog.getByLabel("期限", { exact: true }).click();
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

  await page.getByRole("checkbox", { name: "編集済み課題: 完了" }).click();
  await expect.poll(() => api.tasks[0].is_completed, { timeout: 6_000 }).toBe(true);
  await page.getByPlaceholder("課題を検索 (タイトル/説明)").fill("編集済み");
  await expect(page.getByText("編集済み課題")).toBeVisible();
  await page.getByPlaceholder("課題を検索 (タイトル/説明)").fill("存在しない課題");
  await expect(page.getByText("検索結果がありません")).toBeVisible();
});

test("課題作成時に画像をR2へ直接アップロードする", async ({ page }) => {
  const api = await mockApi(page, { authenticated: true });
  await page.goto("/home");
  await page.getByRole("button", { name: "新規作成" }).first().click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("タイトル").fill("画像つき課題");
  await dialog.getByLabel("サブジェクト", { exact: true }).click();
  await page.getByRole("menuitem", { name: "数学" }).click();
  await dialog.getByLabel("期限", { exact: true }).click();
  await page.getByRole("button", { name: "日付", exact: true }).click();
  await page.locator(".rdp-today button").click();
  await page.getByRole("button", { name: "適用" }).click();
  await dialog.locator('input[type="file"]').setInputFiles({
    name: "proof.png",
    mimeType: "image/png",
    buffer: Buffer.from([0x89, 0x50, 0x4e, 0x47]),
  });
  await dialog.getByRole("button", { name: "作成" }).click();

  await expect(page.getByText("画像つき課題")).toBeVisible();
  await expect.poll(() => api.uploadedKeys.length).toBe(1);
  await expect.poll(() => api.tasks[0].image_keys?.[0]).toContain("550e8400");
});

test("課題編集で画像を複数ドラッグ&ドロップして保存する", async ({ page }) => {
  const api = await mockApi(page, { authenticated: true });
  await page.goto("/home");
  await page.getByRole("button", { name: "新規作成" }).first().click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("タイトル").fill("編集画像テスト");
  await dialog.getByLabel("サブジェクト", { exact: true }).click();
  await page.getByRole("menuitem", { name: "数学" }).click();
  await dialog.getByLabel("期限", { exact: true }).click();
  await page.getByRole("button", { name: "日付", exact: true }).click();
  await page.locator(".rdp-today button").click();
  await page.getByRole("button", { name: "適用" }).click();
  await dialog.getByRole("button", { name: "作成" }).click();

  await page.getByRole("button", { name: "アクション" }).click();
  await page.getByRole("button", { name: "編集" }).click();
  await page.evaluate(() => {
    const target = document.querySelector('[data-testid="ed-2-image-dropzone"]');
    const transfer = new DataTransfer();
    transfer.items.add(new File([new Uint8Array([1])], "first.png", { type: "image/png" }));
    transfer.items.add(new File([new Uint8Array([2])], "second.webp", { type: "image/webp" }));
    target?.dispatchEvent(new DragEvent("drop", { bubbles: true, dataTransfer: transfer }));
  });
  await expect(page.getByRole("button", { name: /画像を削除/ })).toHaveCount(2);
  await page.getByRole("button", { name: "保存" }).click();

  await expect.poll(() => api.uploadedKeys.length).toBe(2);
  await expect.poll(() => api.tasks[0].image_keys?.length).toBe(2);
  await expect(page.getByRole("img", { name: /編集画像テスト - 画像/ })).toHaveCount(2);
});

test("課題の詳細に入力した改行を一覧表示でも保持する", async ({ page }) => {
  const api = await mockApi(page, { authenticated: true });
  await page.goto("/home");
  await expect(page.locator('astro-island[component-url*="TasksList"]')).not.toHaveAttribute("ssr", "");

  await page.getByRole("button", { name: "新規作成" }).first().click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("タイトル").fill("改行を含む課題");
  await dialog.getByLabel("サブジェクト", { exact: true }).click();
  await page.getByRole("menuitem", { name: "数学" }).click();
  await dialog.getByLabel("期限", { exact: true }).click();
  await page.getByRole("button", { name: "日付", exact: true }).click();
  await page.locator(".rdp-today button").click();
  await page.getByRole("button", { name: "適用" }).click();
  await dialog.getByLabel("説明 （任意）").fill("1行目\n2行目");
  await dialog.getByRole("button", { name: "作成" }).click();

  const description = page.locator("p.whitespace-pre-line", { hasText: "1行目" });
  await expect(description).toBeVisible();
  await expect(description).toHaveCSS("white-space", "pre-line");
  await expect(description).toHaveJSProperty("innerText", "1行目\n2行目");
  await expect.poll(() => api.tasks[0]?.description).toBe("1行目\n2行目");
});

test("課題検索欄と新規作成ボタンの名前を日本語・英語と両画面幅で取得できる", async ({ page }) => {
  await mockApi(page, { authenticated: true });
  await page.goto("/home");
  await expect(page.locator('astro-island[component-url*="TasksList"]')).not.toHaveAttribute("ssr", "");
  const toolbar = page.locator('[data-slot="input-group"]').locator("xpath=../..");

  for (const [lang, searchName, createName] of [
    ["ja", "課題を検索", "新規作成"],
    ["en", "Search tasks", "New task"],
  ] as const) {
    await page.evaluate((value) => {
      localStorage.setItem("display_lang", value);
      window.dispatchEvent(new Event("languagechange"));
    }, lang);

    for (const width of [1280, 375]) {
      await page.setViewportSize({ width, height: 800 });
      await expect(page.getByRole("textbox", { name: searchName, exact: true })).toBeVisible();
      await expect(toolbar.getByRole("button", { name: createName, exact: true })).toBeVisible();
    }
  }
});

test("完了状態フィルターをキーボードで切り替え、選択と解除を読み取れる", async ({ page }) => {
  const api = await mockApi(page);
  api.tasks.push(
    { id: 10, title: "未完了の課題", subject_id: 1, deadline: "", is_completed: false },
    { id: 11, title: "完了済みの課題", subject_id: 1, deadline: "", is_completed: true },
  );
  await login(page);

  const filters = page.getByRole("group", { name: "完了状態で絞り込み" });
  const incomplete = filters.getByRole("button", { name: "未完了", exact: true });
  const completed = filters.getByRole("button", { name: "完了済み", exact: true });
  await expect(filters).not.toContainText("選択中の条件をもう一度押すと、すべて表示します。");
  await expect(filters.getByRole("status")).toHaveText("表示中: すべて");
  await expect(incomplete).toHaveAttribute("aria-pressed", "false");
  await expect(completed).toHaveAttribute("aria-pressed", "false");

  await page.getByPlaceholder("課題を検索 (タイトル/説明)").focus();
  await page.keyboard.press("Tab");
  await expect(page.getByRole("button", { name: "新規作成" }).first()).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(incomplete).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(incomplete).toHaveAttribute("aria-pressed", "true");
  await expect(filters.getByRole("status")).toHaveText("表示中: 未完了");
  await expect(page.getByText("未完了の課題")).toBeVisible();
  await expect(page.getByText("完了済みの課題")).toHaveCount(0);

  await page.keyboard.press("Tab");
  await expect(completed).toBeFocused();
  await page.keyboard.press("Space");
  await expect(incomplete).toHaveAttribute("aria-pressed", "false");
  await expect(completed).toHaveAttribute("aria-pressed", "true");
  await expect(filters.getByRole("status")).toHaveText("表示中: 完了済み");
  await expect(page.getByText("完了済みの課題")).toBeVisible();
  await expect(page.getByText("未完了の課題")).toHaveCount(0);

  await page.keyboard.press("Space");
  await expect(completed).toHaveAttribute("aria-pressed", "false");
  await expect(filters.getByRole("status")).toHaveText("表示中: すべて");
  await expect(page.getByText("未完了の課題")).toBeVisible();
  await expect(page.getByText("完了済みの課題")).toBeVisible();

  await page.evaluate(() => {
    localStorage.setItem("display_lang", "en");
    window.dispatchEvent(new Event("languagechange"));
  });
  const englishFilters = page.getByRole("group", { name: "Filter by completion status" });
  await expect(englishFilters.getByRole("button", { name: "Incomplete" })).toHaveAttribute("aria-pressed", "false");
  await expect(englishFilters.getByRole("button", { name: "Completed" })).toHaveAttribute("aria-pressed", "false");
  await expect(englishFilters.getByRole("status")).toHaveText("Showing: All");
  await expect(englishFilters).not.toContainText("Press the selected filter again to show all tasks.");
  await englishFilters.getByRole("button", { name: "Completed" }).click();
  await expect(englishFilters.getByRole("status")).toHaveText("Showing: Completed");
  await englishFilters.getByRole("button", { name: "Completed" }).click();
  await expect(englishFilters.getByRole("status")).toHaveText("Showing: All");
});

test("モバイルの新規作成ボタンを名前で見つけて作成画面を開ける", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockApi(page);
  await login(page);

  const createButton = page.getByRole("button", { name: "新規作成", exact: true })
    .and(page.locator('button[aria-label="新規作成"]'));
  await expect(createButton).toBeVisible();
  await createButton.click();
  await expect(page).toHaveURL(/\/tasks\/new$/);
  await expect(page.getByRole("heading", { name: "新規課題の作成" })).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("TOTP無効化とリカバリーコード再発行は確認後だけ実行する", async ({ page }) => {
  await mockApi(page, { authenticated: true });
  let totpDisableCalls = 0;
  let recoveryGenerateCalls = 0;
  await page.route("http://localhost:3063/graphql", async (route) => {
    const query = route.request().postDataJSON()?.query || "";
    const respond = (data: unknown) => route.fulfill({ json: { data } });
    if (query.includes("authSettings{")) {
      return respond({
        authSettings: {
          email_login_enabled: true,
          has_password: true,
          totp_enabled: true,
          passkey_count: 0,
          recovery_codes_count: 2,
        },
        myPasskeys: [],
      });
    }
    if (query.includes("totpDisable")) {
      totpDisableCalls += 1;
      return respond({ totpDisable: true });
    }
    if (query.includes("recoveryGenerate")) {
      recoveryGenerateCalls += 1;
      return respond({ recoveryGenerate: ["code-one", "code-two"] });
    }
    return route.fallback();
  });
  await page.goto("/settings/security");

  await page.getByRole("button", { name: "無効化", exact: true }).click();
  await expect(page.getByRole("alertdialog").getByText("TOTPを無効化しますか？")).toBeVisible();
  expect(totpDisableCalls).toBe(0);
  await page.getByRole("button", { name: "キャンセル" }).click();
  expect(totpDisableCalls).toBe(0);

  await page.getByRole("button", { name: "無効化", exact: true }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "無効化" }).focus();
  await page.keyboard.press("Enter");
  await expect.poll(() => totpDisableCalls).toBe(1);

  await page.getByRole("button", { name: "リカバリーコードを再発行" }).click();
  await expect(page.getByRole("alertdialog").getByText("リカバリーコードを再発行しますか？")).toBeVisible();
  expect(recoveryGenerateCalls).toBe(0);
  await page.getByRole("alertdialog").getByRole("button", { name: "リカバリーコードを再発行" }).click();
  await expect.poll(() => recoveryGenerateCalls).toBe(1);
  await expect(page.getByText("code-one")).toBeVisible();
});

test("モバイルの設定メニューをロゴ横の明確なボタンから開ける", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockApi(page, { authenticated: true });
  await page.goto("/settings");

  const menuButton = page.getByRole("button", { name: "設定メニューを開く" });
  await expect(menuButton).toBeVisible();
  await expect(menuButton).toContainText("メニュー");
  const box = await menuButton.boundingBox();
  expect(box?.y).toBeLessThan(60);
  expect(box?.width).toBeGreaterThanOrEqual(72);

  await menuButton.click();
  const mobileSidebar = page.locator('[data-mobile="true"]');
  await expect(mobileSidebar).toBeVisible();
  await expect(mobileSidebar.getByRole("link", { name: "セキュリティ設定" })).toBeVisible();
});

test("ヘッダーロゴは認証確認中も有効で、ログイン確認後にホームを指す", async ({ page }) => {
  await mockApi(page, { authenticated: true, meDelayMs: 700 });
  await page.goto("/platform/android");

  const logo = page.locator("header").getByRole("link", { name: "Taskly" });
  await expect(logo).toBeVisible();
  await expect(logo).toHaveAttribute("href", "/");
  await expect(logo).toHaveAttribute("href", "/home", { timeout: 5_000 });
});

test("未ログイン時のヘッダーロゴは公開トップを指す", async ({ page }) => {
  await mockApi(page, { authenticated: false });
  await page.goto("/platform/android");

  const logo = page.locator("header").getByRole("link", { name: "Taskly" });
  await expect(logo).toBeVisible();
  await expect(logo).toHaveAttribute("href", "/");
});

test("課題ごとの完了チェックボックスをキーボードで操作でき、他のボタンは状態を変えない", async ({ page }) => {
  const api = await mockApi(page, { authenticated: true });
  api.tasks.push(
    { id: 1, title: "数学の課題", subject_id: 1, deadline: "2099-12-01", is_completed: false },
    { id: 2, title: "英語の課題", subject_id: 1, deadline: "2099-12-01", is_completed: true },
  );
  await page.goto("/home");
  await expect(page.locator('astro-island[component-url*="TasksList"]')).not.toHaveAttribute("ssr", "");

  const math = page.getByRole("checkbox", { name: "数学の課題: 完了" });
  const english = page.getByRole("checkbox", { name: "英語の課題: 完了" });
  await expect(math).not.toBeChecked();
  await expect(english).toBeChecked();
  await math.focus();
  await page.keyboard.press("Shift+Tab");
  await page.keyboard.press("Tab");
  await expect(math).toBeFocused();
  expect(await math.evaluate((element) => element.matches(":focus-visible"))).toBe(true);
  expect(await math.evaluate((element) => getComputedStyle(element).boxShadow)).not.toBe("none");
  await page.keyboard.press("Space");
  await expect(math).toBeChecked();
  await expect.poll(() => api.tasks[0].is_completed, { timeout: 6_000 }).toBe(true);

  await english.focus();
  await expect(english).toBeFocused();
  expect(await english.evaluate((element) => element.matches(":focus-visible"))).toBe(true);
  expect(await english.evaluate((element) => getComputedStyle(element).boxShadow)).not.toBe("none");
  await page.keyboard.press("Space");
  await expect(english).not.toBeChecked();
  await expect.poll(() => api.tasks[1].is_completed, { timeout: 6_000 }).toBe(false);

  await page.getByRole("button", { name: "アクション" }).first().click();
  await expect(math).toBeChecked();
  await page.keyboard.press("Escape");
  await page.getByText("数学の課題").click();
  await expect(math).not.toBeChecked();
});

test("API 失敗を表示し、再読み込みで復帰する", async ({ page }) => {
  await mockApi(page, { failFirstSearch: true });
  await login(page);
  await expect(page.getByText("temporary outage", { exact: true })).toBeVisible();
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
  await expect(page.getByRole("heading", { name: "Connect to Taskly" })).toBeVisible();
  await expect(page.getByText("https://mcp.taskly.iy0306.com", { exact: true })).toBeVisible();
  await expect(page.getByText("TASKLY_MCP_TOKEN", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Update expiration" }).click();
  await expect(page.getByRole("dialog").getByText("Expiration (optional)")).toBeVisible();
  await page.evaluate(() => {
    localStorage.setItem("display_lang", "ja");
    window.dispatchEvent(new Event("languagechange"));
  });
  await expect(page.getByRole("dialog").getByText("有効期限(任意)")).toBeVisible();

  await page.goto("/guide/mcp");
  await expect(page.getByRole("heading", { name: "接続までの流れ" })).toBeVisible();
  await expect(page.getByText("https://mcp.taskly.iy0306.com", { exact: true }).last()).toBeVisible();
  await expect(page.getByText("接続できないとき", { exact: true })).toBeVisible();
});


for (const width of [320, 390]) {
  test(`スマホ${width}pxで長い課題をスクロールして登録できる`, async ({ page }) => {
    await page.setViewportSize({ width, height: 640 });
    const api = await mockApi(page, { authenticated: true });
    await page.goto("/tasks/new");
    await page.getByLabel("タイトル", { exact: true }).fill("Teamsで知らされた課題");
    await expect(page.getByRole("button", { name: "新規作成", exact: true })).toBeDisabled();
    await page.getByLabel("サブジェクト").click();
    await page.getByRole("menuitem", { name: "数学" }).click();
    await page.getByLabel("期限", { exact: true }).click();
    await page.getByLabel("日付", { exact: true }).fill("2099-10-16");
    await page.getByRole("button", { name: "範囲選択" }).click();
    await page.getByRole("button", { name: "5限", exact: true }).click();
    await page.getByRole("button", { name: "6限", exact: true }).click();
    await page.getByRole("button", { name: "適用" }).click();
    await expect(page.getByLabel("期限", { exact: true })).toContainText("5〜6限まで");
    await page.getByLabel("期限", { exact: true }).click();
    await page.getByLabel("日付", { exact: true }).fill("2099-10-17");
    await page.getByRole("button", { name: "閉じる", exact: true }).click();
    await expect(page.getByLabel("期限", { exact: true })).toContainText("2099-10-16");

    await page.getByText("詳細を追加（任意）", { exact: true }).click();
    await page.getByLabel("説明 （任意）").fill("長い説明\n".repeat(100));
    await page.locator('input[type="file"]').setInputFiles({ name: "notice.png", mimeType: "image/png", buffer: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aWQAAAABJRU5ErkJggg==", "base64") });
    // Reduced available height models the layout constraint while a phone keyboard is open.
    await page.setViewportSize({ width, height: 360 });
    await page.getByLabel("タイトル", { exact: true }).focus();
    await expect(page.getByLabel("タイトル", { exact: true })).toBeInViewport();
    const submit = page.getByRole("button", { name: "新規作成", exact: true });
    await submit.scrollIntoViewIfNeeded();
    await expect(submit).toBeInViewport();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.screenshot({ path: `test-results/mobile-task-${width}.png`, fullPage: true });
    await submit.click();
    await expect(page).toHaveURL(/\/home$/);
    await expect(page.getByText("Teamsで知らされた課題", { exact: true })).toBeVisible();
    expect(api.tasks).toHaveLength(1);
    expect(api.tasks[0].description).toContain("長い説明\n");
    expect(api.uploadedKeys).toHaveLength(1);
  });
}

test("登録失敗後も入力を保持し、再試行で一件だけ登録する", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 640 });
  const api = await mockApi(page, { authenticated: true });
  let fail = true;
  await page.route("http://localhost:3063/graphql", async (route) => {
    if (fail && route.request().postDataJSON()?.query.includes("createTask(")) {
      fail = false;
      return route.fulfill({ status: 503, headers: corsHeaders, json: { errors: [{ message: "temporary outage" }] } });
    }
    return route.fallback();
  });
  await page.goto("/tasks/new");
  await page.getByLabel("タイトル", { exact: true }).fill("再試行の課題");
  await page.getByLabel("サブジェクト").click();
  await page.getByRole("menuitem", { name: "数学" }).click();
  await page.getByLabel("期限", { exact: true }).click();
  await page.getByLabel("日付", { exact: true }).fill("2099-10-16");
  await page.getByRole("button", { name: "適用" }).click();
  await page.getByRole("button", { name: "新規作成", exact: true }).click();
  await expect(page.getByText("temporary outage", { exact: true })).toBeVisible();
  await expect(page.getByLabel("タイトル", { exact: true })).toHaveValue("再試行の課題");
  await expect(page).toHaveURL(/\/tasks\/new$/);
  await page.getByRole("button", { name: "新規作成", exact: true }).click();
  await expect(page).toHaveURL(/\/home$/);
  expect(api.tasks).toHaveLength(1);
});


test("小テストの完了操作を無効化し、課題ごとに上書きできる", async ({ page }) => {
  const api = await mockApi(page, { authenticated: true });
  await page.goto("/home");
  await page.getByRole("button", { name: "新規作成" }).first().click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("タイトル").fill("小テスト課題");
  await dialog.getByLabel("サブジェクト", { exact: true }).click();
  await page.getByRole("menuitem", { name: "数学" }).click();
  await dialog.getByLabel("期限", { exact: true }).click();
  await page.getByRole("button", { name: "日付", exact: true }).click();
  await page.locator(".rdp-today button").click();
  await page.getByRole("button", { name: "適用" }).click();
  await dialog.getByLabel("タイプ", { exact: true }).click();
  await page.getByRole("option", { name: "小テスト", exact: true }).click();
  await expect(dialog.getByRole("switch", { name: "完了マークを無効にする" })).toBeChecked();
  await dialog.getByRole("button", { name: "新規作成", exact: true }).click();
  await expect.poll(() => api.tasks.length).toBe(1);
  await expect(page.getByRole("checkbox", { name: "小テスト課題: 完了" })).toHaveCount(0);
  const completions: string[] = [];
  page.on("request", (request) => {
    if (request.url().endsWith("/graphql") && /completeTask/.test(request.postData() || "")) completions.push(request.postData()!);
  });
  await page.getByText("小テスト課題", { exact: true }).click();
  await page.waitForTimeout(3300);
  expect(completions).toHaveLength(0);
  await page.getByRole("button", { name: "アクション" }).click();
  await page.getByRole("button", { name: "編集", exact: true }).click();
  await page.getByRole("switch", { name: "完了マークを無効にする" }).click();
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await page.getByRole("checkbox", { name: "小テスト課題: 完了" }).click();
  await expect.poll(() => api.tasks[0].is_completed, { timeout: 6000 }).toBe(true);
});

test("管理者がタイプ別の完了マーク初期値を保存できる", async ({ page }) => {
  await mockApi(page, { authenticated: true, admin: true });
  await page.goto("/admin/task-defaults");
  const quiz = page.getByRole("switch", { name: "小テスト — 完了マークを無効にする" });
  await expect(quiz).toBeChecked();
  await quiz.click();
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByText("更新しました", { exact: true })).toBeVisible();
  await page.reload();
  await expect(quiz).not.toBeChecked();
});

test("編集フォームは保存済みの課題タイプを表示し、言語変更後も同じ値を保存する", async ({ page }) => {
  const api = await mockApi(page, { authenticated: true });
  api.tasks.push({ id: 42, title: "タイプ表示テスト", subject_id: 1, deadline: "2099-10-15", type: "ONLINE", is_completed: false });
  await page.goto("/home");
  await expect(page.getByText("タイプ表示テスト")).toBeVisible();
  await page.getByRole("button", { name: "アクション" }).click();
  await page.getByRole("button", { name: "編集", exact: true }).click();
  const type = page.locator("#ed-42-type");
  await expect(type).toHaveText("オンライン");
  await page.evaluate(() => { localStorage.setItem("display_lang", "en"); window.dispatchEvent(new Event("languagechange")); });
  await expect(type).toHaveText("Online");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect.poll(() => api.tasks[0].type).toBe("ONLINE");
  await page.getByRole("button", { name: "Actions", exact: true }).click();
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  await type.click();
  await page.getByRole("option", { name: "Quiz", exact: true }).click();
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect.poll(() => api.tasks[0].type).toBe("QUIZ");
});


test("画像ごとの進捗と失敗を表示し、全転送後に失敗画像だけ再試行する", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 800 });
  const api = await mockApi(page, { authenticated: true });
  await page.addInitScript(() => {
    (window as any).taskImageRequests = [];
    const send = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.send = function (body) {
      (window as any).taskImageRequests.push(this);
      return send.call(this, body);
    };
  });
  let release!: () => void;
  const slowUpload = new Promise<void>((resolve) => { release = resolve; });
  let uploads = 0;
  await page.route("https://r2.example.test/signed-upload-*", async (route) => {
    if (route.request().method() !== "PUT") return route.fallback();
    uploads++;
    if (route.request().url().endsWith("-1")) return route.fulfill({ status: 503, headers: corsHeaders, body: "" });
    if (route.request().url().endsWith("-2")) {
      await slowUpload;
      return route.fulfill({ status: 200, headers: corsHeaders, body: "" });
    }
    return route.fallback();
  });
  await page.goto("/tasks/new");
  await page.getByLabel("タイトル", { exact: true }).fill("画像再試行テスト");
  await page.getByLabel("サブジェクト").click();
  await page.getByRole("menuitem", { name: "数学" }).click();
  await page.getByLabel("期限", { exact: true }).click();
  await page.getByLabel("日付", { exact: true }).fill("2099-10-16");
  await page.getByRole("button", { name: "適用" }).click();
  await page.getByText("詳細を追加（任意）", { exact: true }).click();
  await page.locator('input[type="file"]').setInputFiles([
    { name: "failed.png", mimeType: "image/png", buffer: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aWQAAAABJRU5ErkJggg==", "base64") },
    { name: "slow.png", mimeType: "image/png", buffer: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aWQAAAABJRU5ErkJggg==", "base64") },
  ]);
  const attachments = page.locator('[data-slot="attachment"]');
  await expect(attachments.first()).toHaveAttribute("data-state", "idle");
  await expect(attachments.first().getByRole("status")).toHaveText("アップロード待ち");
  const submit = page.locator('form button[type="submit"]');
  await submit.click();
  await expect(attachments.first()).toHaveAttribute("data-state", "error");
  await expect(attachments.last()).toHaveAttribute("data-state", "uploading");
  await expect(submit).toBeDisabled();
  await expect(page.getByRole("button", { name: /画像を削除/ }).first()).toBeDisabled();
  await page.evaluate(() => {
    const request = (window as any).taskImageRequests[1] as XMLHttpRequest;
    request.upload.dispatchEvent(new ProgressEvent("progress", { lengthComputable: true, loaded: 1, total: 2 }));
  });
  await expect(attachments.last().getByRole("status")).toHaveText("アップロード中 · 50%");
  await expect(attachments.last().getByRole("progressbar")).toHaveAttribute("value", "50");
  await page.evaluate(() => { localStorage.setItem("display_lang", "en"); window.dispatchEvent(new Event("languagechange")); });
  await expect(attachments.last().getByRole("status")).toHaveText("Uploading · 50%");
  await expect(attachments.first().getByRole("status")).toContainText("Upload failed");
  await expect(submit).toBeDisabled();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await attachments.last().scrollIntoViewIfNeeded();
  await page.screenshot({ path: "test-results/image-upload-progress-mobile.png", fullPage: true });
  expect(api.tasks).toHaveLength(0);
  await page.evaluate(() => (window as any).taskImageRequests[1].upload.dispatchEvent(new ProgressEvent("load")));
  await expect(attachments.last()).toHaveAttribute("data-state", "processing");
  release();
  await expect(attachments.last()).toHaveAttribute("data-state", "done");
  await expect(submit).toBeEnabled();
  await submit.click();
  await expect(page).toHaveURL(/\/home$/);
  expect(uploads).toBe(3);
  expect(api.tasks).toHaveLength(1);
  expect(api.tasks[0].image_keys).toEqual([
    "task-images/1/550e8400-e29b-41d4-a716-000000000003",
    "task-images/1/550e8400-e29b-41d4-a716-000000000002",
  ]);
});

test("画像を編集保存中は入力と二重保存を防ぎ、失敗後に再試行できる", async ({ page }) => {
  const api = await mockApi(page, { authenticated: true });
  api.tasks.push({ id: 42, title: "画像編集中", subject_id: 1, deadline: "2099-10-15", is_completed: false });
  let release!: () => void;
  const response = new Promise<void>((resolve) => { release = resolve; });
  let uploads = 0;
  await page.route("https://r2.example.test/signed-upload-*", async (route) => {
    if (route.request().method() !== "PUT") return route.fallback();
    uploads++;
    if (uploads === 1) {
      await response;
      return route.fulfill({ status: 403, headers: corsHeaders, body: "" });
    }
    return route.fallback();
  });
  await page.goto("/home");
  await page.getByRole("button", { name: "アクション" }).click();
  await page.getByRole("button", { name: "編集", exact: true }).click();
  await page.locator('#ed-42-images').setInputFiles({ name: "edit.png", mimeType: "image/png", buffer: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aWQAAAABJRU5ErkJggg==", "base64") });
  const save = page.getByRole("button", { name: "保存", exact: true });
  await save.click();
  const attachment = page.locator('[data-slot="attachment"]');
  await expect(attachment).toHaveAttribute("data-state", "uploading");
  await expect(save).toBeDisabled();
  await expect(page.getByLabel("タイトル", { exact: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "キャンセル", exact: true })).toBeDisabled();
  await page.keyboard.press("ControlOrMeta+s");
  expect(uploads).toBe(1);
  await attachment.scrollIntoViewIfNeeded();
  await page.screenshot({ path: "test-results/image-upload-progress-edit.png", fullPage: true });
  release();
  await expect(attachment).toHaveAttribute("data-state", "error");
  await expect(attachment.getByRole("status")).toContainText("アップロードに失敗しました");
  await expect(save).toBeEnabled();
  await save.click();
  await expect(page.locator('#ed-42-images')).toHaveCount(0);
  expect(uploads).toBe(2);
  expect(api.tasks[0].image_keys).toHaveLength(1);
});

for (const [lang, width] of [["ja", 1280], ["en", 390]] as const) {
  test(`画像プレビュー・保存・コピー (${lang}, ${width}px)`, async ({ page, context }) => {
    await page.setViewportSize({ width, height: 800 });
    const api = await mockApi(page, { authenticated: true });
    await page.addInitScript((language) => localStorage.setItem("display_lang", language), lang);
    const url = "https://r2.example.test/preview.png";
    const mimeType = lang === "ja" ? "image/png" : "image/webp";
    const data = await page.evaluate((mimeType) => {
      const canvas = document.createElement("canvas");
      canvas.width = 1600;
      canvas.height = 1200;
      canvas.getContext("2d")!.fillRect(0, 0, 1600, 1200);
      return canvas.toDataURL(mimeType).split(",")[1];
    }, mimeType);
    const imageResponse = { headers: corsHeaders, contentType: mimeType, body: Buffer.from(data, "base64") };
    await page.route(url, (route) => route.fulfill(imageResponse));
    // The first request in a new tab belongs to the context, before page routes attach.
    await context.route(url, (route) => route.fulfill(imageResponse));
    api.tasks.push({ id: 70, title: "Preview task", subject_id: 1, deadline: "2099-10-15", is_completed: false, image_urls: [url, url + "?broken"] });
    await page.goto("/home");
    const ja = lang === "ja";
    const trigger = page.getByRole("button", { name: ja ? "画像をプレビュー: Preview task - 画像 1" : "Preview image: Preview task - Image 1", exact: true });
    await trigger.focus();
    await page.keyboard.press("Enter");
    const dialog = page.getByRole("dialog");
    const zoomIn = dialog.getByRole("button", { name: ja ? "拡大" : "Zoom in", exact: true });
    const fit = dialog.getByRole("button", { name: ja ? "画面に合わせる" : "Fit to view" });
    const close = dialog.getByRole("button", { name: ja ? "閉じる" : "Close", exact: true });
    await expect(dialog.getByRole("img")).toBeVisible();
    await expect(page).toHaveURL(/\/home$/);
    const initialWidth = await dialog.getByRole("img").evaluate((el) => el.clientWidth);
    await zoomIn.click();
    await expect(fit).toHaveText("125%");
    await expect.poll(async () => Math.abs(await dialog.getByRole("img").evaluate((el) => el.clientWidth) - initialWidth * 1.25)).toBeLessThan(2);
    for (let i = 0; i < 11; i++) await zoomIn.click();
    await expect(zoomIn).toBeDisabled();
    const region = dialog.getByRole("region");
    expect(await region.evaluate((el) => el.scrollWidth > el.clientWidth && el.scrollHeight > el.clientHeight)).toBe(true);
    await region.evaluate((el) => { el.scrollLeft = el.scrollWidth; el.scrollTop = el.scrollHeight; });
    expect(await region.evaluate((el) => el.scrollLeft > 0 && el.scrollTop > 0)).toBe(true);
    await fit.click();
    await dialog.getByRole("button", { name: ja ? "縮小" : "Zoom out", exact: true }).click();
    await expect(fit).toHaveText("75%");
    for (let i = 0; i < 2; i++) await dialog.getByRole("button", { name: ja ? "縮小" : "Zoom out", exact: true }).click();
    await expect(dialog.getByRole("button", { name: ja ? "縮小" : "Zoom out", exact: true })).toBeDisabled();
    await fit.click();
    const downloadPromise = page.waitForEvent("download");
    await dialog.getByRole("button", { name: ja ? "ダウンロード" : "Download", exact: true }).click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe(`task-70-1.${ja ? "png" : "webp"}`);
    expect(await download.failure()).toBeNull();
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    await dialog.getByRole("button", { name: ja ? "画像をコピー" : "Copy image", exact: true }).click();
    await expect(dialog.getByRole("status")).toHaveText(ja ? "コピーしました" : "Copied");
    expect(await page.evaluate(async () => (await (await navigator.clipboard.read())[0].getType("image/png")).size)).toBeGreaterThan(100);
    const popupPromise = page.waitForEvent("popup");
    await dialog.getByRole("link", { name: ja ? "新しいタブで開く" : "Open in new tab" }).click();
    const popup = await popupPromise;
    await expect(popup).toHaveURL(url);
    await popup.close();
    await page.screenshot({ path: `test-results/image-preview-${lang}.png` });
    expect(await dialog.evaluate((el) => el.getBoundingClientRect().right <= innerWidth)).toBe(true);
    await page.keyboard.press("Escape");
    await expect(dialog).not.toBeVisible();
    await expect(trigger).toBeFocused();
    await trigger.click();
    await expect(fit).toHaveText("100%");
    await close.click();
    await expect(dialog).not.toBeVisible();
    await trigger.click();
    await page.locator('[data-slot="dialog-overlay"]').click({ position: { x: 2, y: 2 } });
    await expect(dialog).not.toBeVisible();
    expect(api.tasks[0].is_completed).toBe(false);
    const second = page.getByRole("button", { name: ja ? "画像をプレビュー: Preview task - 画像 2" : "Preview image: Preview task - Image 2", exact: true });
    await second.click();
    await expect(dialog.getByRole("alert")).toBeVisible();
    await close.click();
    await expect(second).toBeFocused();
  });
}

test("画像取得・コピー失敗とコピー非対応をプレビュー内に表示する", async ({ page }) => {
  const api = await mockApi(page, { authenticated: true });
  const url = "https://r2.example.test/error.png";
  let failFetch = false;
  await page.route(url, (route) => route.fulfill(failFetch
    ? { status: 403, headers: corsHeaders, body: "expired" }
    : { headers: corsHeaders, contentType: "image/png", body: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aWQAAAABJRU5ErkJggg==", "base64") }));
  api.tasks.push({ id: 71, title: "Error image", subject_id: 1, deadline: "2099-10-15", is_completed: false, image_urls: [url] });
  await page.goto("/home");
  const trigger = page.getByRole("button", { name: "画像をプレビュー: Error image - 画像 1", exact: true });
  await trigger.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("img")).toBeVisible();
  failFetch = true;
  await dialog.getByRole("button", { name: "ダウンロード", exact: true }).click();
  await expect(dialog.getByRole("status")).toContainText("ダウンロードできませんでした");
  await page.evaluate(() => Object.defineProperty(navigator.clipboard, "write", { configurable: true, value: () => Promise.reject(new Error("Permission denied")) }));
  await dialog.getByRole("button", { name: "画像をコピー", exact: true }).click();
  await expect(dialog.getByRole("status")).toContainText("画像をコピーできませんでした");
  await page.keyboard.press("Escape");
  await page.evaluate(() => Object.defineProperty(window, "ClipboardItem", { configurable: true, value: undefined }));
  await trigger.click();
  await expect(dialog.getByRole("button", { name: "画像をコピー", exact: true })).toBeDisabled();
  await expect(dialog.getByRole("status")).toContainText("このブラウザーは画像コピーに対応していません");
  expect(api.tasks[0].is_completed).toBe(false);
});
