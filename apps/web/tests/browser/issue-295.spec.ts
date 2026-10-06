import { expect, test, type Page } from "@playwright/test";

const api = "http://localhost:3063";
const email = "browser@example.test";
const challenge = "AQIDBAUGBwgJCgsMDQ4PEA";

async function mockLogin(page: Page, methods = ["totp", "recovery"]) {
  let authenticated = false;
  const calls: any[] = [];
  await page.addInitScript(() => localStorage.setItem("display_lang", "ja"));
  await page.route(`${api}/**`, async (route) => {
    const path = new URL(route.request().url()).pathname;
    const body = route.request().postDataJSON?.() || {};
    calls.push({ path, body });
    const reply = (json: unknown, status = 200) => route.fulfill({ status, json });
    if (path === "/auth/options") return reply({ has_password: true, methods: ["password", ...methods] });
    if (path === "/auth/request-code") return reply({ ok: true });
    if (path === "/auth/login") {
      if (body.password) return reply(body.password === "browser-password"
        ? { mfa_required: true, auth_transaction: "test-transaction", methods, expires_in: 300 }
        : { error: "invalid password" }, body.password === "browser-password" ? 200 : 401);
      if (body.totp === "123456" && body.auth_transaction === "test-transaction") {
        authenticated = true;
        return reply({ authenticated: true });
      }
      if ((body.code === "123456" || body.recovery_code === "recovery-ok") && body.auth_transaction === "test-transaction") {
        authenticated = true;
        return reply({ authenticated: true });
      }
      return reply({ error: body.auth_transaction ? "invalid code" : "auth transaction expired" }, 401);
    }
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
  return { calls, setAuthenticated: (value: boolean) => { authenticated = value; } };
}

async function beginPasswordLogin(page: Page) {
  await page.goto("/login");
  await expect(page.locator('astro-island[component-url*="Begin"]')).not.toHaveAttribute("ssr", "");
  await page.getByLabel("メールアドレス").fill(email);
  await page.getByRole("button", { name: "次へ" }).click();
  await expect(page).toHaveURL(/\/login\/password/);
  await expect(page.locator('astro-island[component-url*="Password"]')).not.toHaveAttribute("ssr", "");
  await page.getByLabel("パスワード").fill("browser-password");
  await page.getByRole("button", { name: "次へ" }).click();
}

test("MFA: パスワード後に TOTP を選び、失敗と成功を検証する", async ({ page }) => {
  const { calls } = await mockLogin(page);
  await beginPasswordLogin(page);
  await expect(page).toHaveURL(/\/login\/select/);
  await page.getByRole("button", { name: "認証コード（TOTP）で続行" }).click();
  await expect(page).toHaveURL(/\/login\/totp/);
  await expect(page.locator('astro-island[component-url*="Totp"]')).not.toHaveAttribute("ssr", "");
  const code = page.getByLabel("TOTP");
  await code.fill("000000");
  await page.getByRole("button", { name: "続行" }).click();
  await expect(page.getByText("invalid code")).toBeVisible();
  expect(calls.filter((call) => call.path === "/auth/login").slice(-1)[0]?.body).toMatchObject({ auth_transaction: "test-transaction", totp: "000000" });
  await code.fill("123456");
  await page.getByRole("button", { name: "続行" }).click();
  await expect(page).toHaveURL(/\/home$/);
  expect(calls.filter((call) => call.path === "/auth/login").slice(-1)[0]?.body).toMatchObject({ auth_transaction: "test-transaction", totp: "123456" });
});

test("MFA: 期限切れの取引を破棄し、パスワードへの案内を表示する", async ({ page }) => {
  await mockLogin(page);
  await beginPasswordLogin(page);
  await page.getByRole("button", { name: "認証コード（TOTP）で続行" }).click();
  await expect(page.locator('astro-island[component-url*="Totp"]')).not.toHaveAttribute("ssr", "");
  await page.evaluate(() => sessionStorage.clear());
  await page.getByLabel("TOTP").fill("123456");
  await page.getByRole("button", { name: "続行" }).click();
  await expect(page.getByText("認証の有効期限が切れました。パスワードからやり直してください")).toBeVisible();
});

test("MFA: メールコードを送信し、誤入力と成功を検証する", async ({ page }) => {
  const { calls } = await mockLogin(page, ["email_code", "recovery"]);
  await beginPasswordLogin(page);
  await page.getByRole("button", { name: "メールコードで続行" }).click();
  await expect(page).toHaveURL(/\/login\/mail-code/);
  await expect(page.locator('astro-island[component-url*="MailCode"]')).not.toHaveAttribute("ssr", "");
  expect(calls.some((call) => call.path === "/auth/request-code" && call.body.email === email)).toBe(true);
  const code = page.getByLabel("メールコード");
  await code.fill("000000");
  await page.getByRole("button", { name: "続行" }).click();
  await expect(page.getByText("invalid code")).toBeVisible();
  await code.fill("123456");
  await page.getByRole("button", { name: "続行" }).click();
  await expect(page).toHaveURL(/\/home$/);
});

test("MFA: リカバリーコードの誤入力と成功を検証する", async ({ page }) => {
  const { calls } = await mockLogin(page, ["totp", "recovery"]);
  await beginPasswordLogin(page);
  await page.getByRole("button", { name: "リカバリーコードで続行" }).click();
  await expect(page).toHaveURL(/\/login\/recovery/);
  await expect(page.locator('astro-island[component-url*="Recovery"]')).not.toHaveAttribute("ssr", "");
  const code = page.getByLabel("リカバリーコード");
  await code.fill("recovery-bad");
  await page.getByRole("button", { name: "続行" }).click();
  await expect(page.getByText("invalid code")).toBeVisible();
  await code.fill("recovery-ok");
  await page.getByRole("button", { name: "続行" }).click();
  await expect(page).toHaveURL(/\/home$/);
  expect(calls.filter((call) => call.path === "/auth/login").slice(-1)[0]?.body).toMatchObject({ auth_transaction: "test-transaction", recovery_code: "recovery-ok" });
});

test("パスキー: 仮想認証器で登録し、ログイン成功とサーバー失敗を確認する", async ({ page, context }) => {
  const { calls, setAuthenticated } = await mockLogin(page);
  const client = await context.newCDPSession(page);
  await client.send("WebAuthn.enable");
  await client.send("WebAuthn.addVirtualAuthenticator", { options: {
    protocol: "ctap2", transport: "internal", hasResidentKey: true,
    hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true,
  } });
  let failFinish = false;
  await page.route(`${api}/webauthn/login/**`, async (route) => {
    const path = new URL(route.request().url()).pathname;
    const body = route.request().postDataJSON();
    calls.push({ path, body });
    if (path.endsWith("/begin")) return route.fulfill({ json: { publicKey: { challenge, timeout: 30000, userVerification: "preferred" } } });
    if (failFinish) return route.fulfill({ status: 401, json: { error: "assertion rejected" } });
    setAuthenticated(true);
    return route.fulfill({ json: { authenticated: true } });
  });
  await page.route(`${api}/graphql`, async (route) => {
    const body = route.request().postDataJSON();
    if (body.query.includes("authSettings{")) return route.fulfill({ json: { data: { authSettings: { email_login_enabled: true, has_password: true, totp_enabled: true, passkey_count: 0, recovery_codes_count: 0 }, myPasskeys: [] } } });
    if (body.query.includes("webauthnRegisterBegin")) return route.fulfill({ json: { data: { webauthnRegisterBegin: JSON.stringify({ publicKey: {
      challenge, rp: { name: "Taskly" }, user: { id: "AQIDBA", name: email, displayName: "Browser Tester" },
      pubKeyCredParams: [{ type: "public-key", alg: -7 }], authenticatorSelection: { residentKey: "required", userVerification: "preferred" }, timeout: 30000,
    } }) } } });
    if (body.query.includes("webauthnRegisterFinish")) {
      calls.push({ path: "/webauthn/register/finish", body });
      return route.fulfill({ json: { data: { webauthnRegisterFinish: true } } });
    }
    return route.fallback();
  });
  setAuthenticated(true);
  await page.goto("/settings/security");
  await expect(page.locator('astro-island[component-url*="Security"]')).not.toHaveAttribute("ssr", "");
  await expect(page.getByRole("button", { name: "無効化", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "パスキーを登録", exact: true }).click();
  await expect.poll(() => calls.filter((call) => call.path === "/webauthn/register/finish").length).toBe(1);
  await expect(page.getByText("パスキーを登録しました", { exact: true })).toBeVisible();
  const registration = calls.find((call) => call.path === "/webauthn/register/finish");
  expect(JSON.parse(registration.body.variables.input)).toMatchObject({ type: "public-key", response: { attestationObject: expect.any(String), clientDataJSON: expect.any(String) } });

  failFinish = true;
  setAuthenticated(false);
  await page.goto(`/login/passkey?email=${encodeURIComponent(email)}`);
  await expect(page.getByText("assertion rejected")).toBeVisible();
  await expect(page).toHaveURL(/\/login\/passkey/);
  const assertion = calls.find((call) => call.path === "/webauthn/login/finish");
  expect(assertion?.body).toMatchObject({ email, type: "public-key", response: { signature: expect.any(String), authenticatorData: expect.any(String) } });
  failFinish = false;
  await page.getByRole("button", { name: "パスキーで続行" }).click();
  await expect(page).toHaveURL(/\/home$/);
});

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
  const { setAuthenticated } = await mockLogin(page);
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
  const { setAuthenticated } = await mockLogin(page);
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
  const { setAuthenticated } = await mockLogin(page);
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

test("モバイル幅でログイン・課題作成と完了・設定への移動を操作できる", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockLogin(page);
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
  await beginPasswordLogin(page);
  await page.getByRole("button", { name: "認証コード（TOTP）で続行" }).click();
  await expect(page.locator('astro-island[component-url*="Totp"]')).not.toHaveAttribute("ssr", "");
  await page.getByLabel("TOTP").fill("123456");
  await page.getByRole("button", { name: "続行" }).click();
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
