import { expect, test, type Page } from "@playwright/test";

type Task = {
  id: number;
  title: string;
  subject_id: number;
  deadline: string;
  is_completed: boolean;
};

async function mockApi(page: Page, options: { admin?: boolean; failFirstSearch?: boolean } = {}) {
  await page.addInitScript(() => localStorage.setItem("display_lang", "ja"));
  let authenticated = false;
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
