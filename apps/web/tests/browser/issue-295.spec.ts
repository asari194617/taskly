import { expect, test, type Page } from "@playwright/test";

const api = "http://localhost:3063";
const email = "browser@example.test";

async function mockApi(page: Page) {
  let authenticated = false;
  await page.addInitScript(() => localStorage.setItem("display_lang", "ja"));
  await page.route(`${api}/**`, async (route) => {
    const path = new URL(route.request().url()).pathname;
    const body = route.request().postDataJSON?.() || {};
    const reply = (json: unknown, status = 200) => route.fulfill({ status, json });
    if (path === "/graphql") {
      const query = body.query || "";
      if (query.includes("me{")) return reply(authenticated
        ? { data: { me: { id: 1, email, display_name: "Browser Tester", is_agreed_policies: true, is_admin: false, is_manager: false } } }
        : { errors: [{ message: "unauthorized" }] }, authenticated ? 200 : 401);
      if (query.includes("subjects{")) return reply({ data: { subjects: [{ id: 1, name_ja: "数学", name_en: "Math" }] } });
      if (query.includes("tasks(")) return reply({ data: { tasks: { items: [], next_cursor: null } } });
      throw new Error(`Unmocked GraphQL operation: ${query}`);
    }
    return reply({ error: `unexpected API path: ${path}` }, 404);
  });
  return { setAuthenticated: (value: boolean) => { authenticated = value; } };
}

async function mockPushBrowser(page: Page, initialPermission: "default" | "granted" | "denied", requestedPermission = initialPermission) {
  await page.addInitScript(({ initialPermission, requestedPermission }) => {
    let permission = initialPermission;
    let subscription: any = null;
    Object.defineProperty(window, "PushManager", { configurable: true, value: function PushManager() {} });
    Object.defineProperty(window.Notification, "permission", { configurable: true, get: () => permission });
    Object.defineProperty(window.Notification, "requestPermission", { configurable: true, value: async () => { permission = requestedPermission; return permission; } });
    const pushManager = {
      getSubscription: async () => subscription,
      subscribe: async (options: PushSubscriptionOptionsInit) => {
        subscription = {
          endpoint: "https://push.example.test/device",
          options: { applicationServerKey: options.applicationServerKey },
          toJSON: () => ({ keys: { p256dh: "test-p256dh", auth: "test-auth" } }),
          unsubscribe: async () => { subscription = null; return true; },
        };
        return subscription;
      },
    };
    Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: { ready: Promise.resolve({ pushManager }), register: async () => ({ pushManager }), getRegistration: async () => ({ pushManager }) } });
  }, { initialPermission, requestedPermission });
}

test("Web Push: 購読・解除と API 失敗時の表示を検証する", async ({ page }) => {
  const { setAuthenticated } = await mockApi(page);
  setAuthenticated(true);
  await mockPushBrowser(page, "granted");
  const changes: string[] = [];
  let failRegistration = false;
  await page.route(`${api}/**`, async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/settings/notification-methods") return route.fulfill({ json: { email: true, webpush: false } });
    if (path === "/settings/notification-types") return route.fulfill({ json: { TASK: true } });
    if (path === "/push/devices") return route.fulfill({ json: [] });
    if (path === "/push/vapidPublicKey") return route.fulfill({ json: { publicKey: "AQIDBA" } });
    if (path === "/push/subscribe" || path === "/push/unsubscribe") {
      changes.push(path);
      expect(route.request().postDataJSON().endpoint).toBe("https://push.example.test/device");
      return route.fulfill({ status: path === "/push/subscribe" && failRegistration ? 500 : 200, json: {} });
    }
    return route.fallback();
  });
  await page.goto("/settings/notifications");
  await expect(page.locator('astro-island[component-url*="Notifications"]')).not.toHaveAttribute("ssr", "");
  await page.getByRole("button", { name: "通知方法" }).click();
  const push = page.getByRole("switch", { name: "Webプッシュ通知" });
  await push.click();
  await expect(push).toHaveAttribute("aria-checked", "true");
  await expect(page.getByText("プッシュ通知を有効化しました")).toBeVisible();
  await push.click();
  await expect(push).toHaveAttribute("aria-checked", "false");
  await expect(page.getByText("プッシュ通知を無効化しました")).toBeVisible();
  expect(changes).toEqual(["/push/subscribe", "/push/unsubscribe"]);
  failRegistration = true;
  await push.click();
  await expect(page.getByText("サーバー登録に失敗しました")).toBeVisible();
  await expect(push).toHaveAttribute("aria-checked", "false");
});

test("Web Push: 通知を拒否したブラウザでは購読できない", async ({ page }) => {
  const { setAuthenticated } = await mockApi(page);
  setAuthenticated(true);
  await mockPushBrowser(page, "denied");
  await page.route(`${api}/**`, async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/settings/notification-methods") return route.fulfill({ json: { email: true, webpush: false } });
    if (path === "/settings/notification-types") return route.fulfill({ json: { TASK: true } });
    if (path === "/push/devices") return route.fulfill({ json: [] });
    if (path === "/push/vapidPublicKey") return route.fulfill({ json: { publicKey: "AQIDBA" } });
    if (path === "/push/subscribe") throw new Error("Denied permission must not subscribe");
    return route.fallback();
  });
  await page.goto("/settings/notifications");
  await expect(page.locator('astro-island[component-url*="Notifications"]')).not.toHaveAttribute("ssr", "");
  await page.getByRole("button", { name: "通知方法" }).click();
  await expect(page.getByRole("switch", { name: "Webプッシュ通知" })).toBeDisabled();
  await expect(page.getByText("通知がブラウザでブロックされています。ブラウザのサイト設定から通知を許可してください。")).toBeVisible();
});

test("Web Push: 許可ダイアログで拒否した場合は購読せず、案内を表示する", async ({ page }) => {
  const { setAuthenticated } = await mockApi(page);
  setAuthenticated(true);
  await mockPushBrowser(page, "default", "denied");
  let subscribeCalls = 0;
  await page.route(`${api}/**`, async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/settings/notification-methods") return route.fulfill({ json: { email: true, webpush: false } });
    if (path === "/settings/notification-types") return route.fulfill({ json: { TASK: true } });
    if (path === "/push/devices") return route.fulfill({ json: [] });
    if (path === "/push/vapidPublicKey") return route.fulfill({ json: { publicKey: "AQIDBA" } });
    if (path === "/push/subscribe") subscribeCalls++;
    return route.fallback();
  });
  await page.goto("/settings/notifications");
  await expect(page.locator('astro-island[component-url*="Notifications"]')).not.toHaveAttribute("ssr", "");
  await page.getByRole("button", { name: "通知方法" }).click();
  const push = page.getByRole("switch", { name: "Webプッシュ通知" });
  await push.click();
  await expect(page.getByText("通知が許可されていません", { exact: true })).toBeVisible();
  await expect(push).toHaveAttribute("aria-checked", "false");
  expect(subscribeCalls).toBe(0);
});

test("モバイル幅で課題作成と完了・設定への移動を操作できる", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const { setAuthenticated } = await mockApi(page);
  setAuthenticated(true);
  const tasks: Array<{ id: number; title: string; subject_id: number; deadline: string; is_completed: boolean }> = [];
  await page.route(`${api}/graphql`, async (route) => {
    const { query, variables = {} } = route.request().postDataJSON();
    if (query.includes("createTask(")) {
      tasks.push({ id: 1, title: variables.title, subject_id: variables.subject_id, deadline: variables.deadline, is_completed: false });
      return route.fulfill({ json: { data: { createTask: tasks[0] } } });
    }
    if (query.includes("completeTask(")) {
      tasks[0].is_completed = true;
      return route.fulfill({ json: { data: { completeTask: true } } });
    }
    if (query.includes("tasks(")) return route.fulfill({ json: { data: { tasks: { items: tasks, next_cursor: null } } } });
    return route.fallback();
  });
  await page.goto("/home");
  await expect(page).toHaveURL(/\/home$/);
  await page.getByRole("button", { name: "新規作成", exact: true }).and(page.locator('button[aria-label="新規作成"]')).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("タイトル").fill("モバイル課題");
  await dialog.getByRole("button", { name: "選択してください" }).first().click();
  await page.getByRole("menuitem", { name: "数学" }).click();
  await dialog.getByRole("button", { name: "未設定" }).click();
  await page.getByRole("button", { name: "日付", exact: true }).click();
  await page.locator(".rdp-today button").click();
  await page.getByRole("button", { name: "適用" }).click();
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(page.getByText("モバイル課題")).toBeVisible();
  const completion = page.getByRole("checkbox", { name: "モバイル課題: 完了" });
  await completion.click();
  await expect(completion).toBeChecked();
  await expect.poll(() => tasks[0]?.is_completed, { timeout: 8_000 }).toBe(true);
  await page.getByRole("button", { name: "Open user menu" }).click();
  await page.getByRole("menuitem", { name: "設定" }).click();
  await expect(page).toHaveURL(/\/settings$/);
  await page.getByRole("link", { name: /通知設定/ }).click();
  await expect(page).toHaveURL(/\/settings\/notifications$/);
  await page.getByRole("button", { name: "設定メニューを開く" }).click();
  await expect(page.locator('[data-mobile="true"]').getByRole("link", { name: "セキュリティ設定" })).toBeVisible();
});
