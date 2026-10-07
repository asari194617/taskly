import { expect, test } from "@playwright/test";

test("OAuth ログインへ追加のクリックなしで進み、next を引き継ぐ", async ({ page }) => {
  test.skip(!process.env.PUBLIC_AUTH_OAUTH_START_URL, "OAuth 設定時のみ実行");

  const oauthStart = new URL(process.env.PUBLIC_AUTH_OAUTH_START_URL!);
  await page.route(`${oauthStart.origin}${oauthStart.pathname}**`, (route) =>
    route.fulfill({ status: 200, contentType: "text/html", body: "OAuth start reached" }),
  );

  await page.goto("/");
  await page.getByRole("link", { name: "ログイン", exact: true }).click();
  await expect(page).toHaveURL(oauthStart.toString());

  await page.goto("/login?next=%2Fsettings%3Ftab%3Dsecurity");
  await expect(page).toHaveURL((url) =>
    url.origin === oauthStart.origin &&
    url.pathname === oauthStart.pathname &&
    url.searchParams.get("next") === "/settings?tab=security",
  );
});

test("旧ログインURLからもOAuthへ進み、メールアドレスは引き継がない", async ({ page }) => {
  test.skip(!process.env.PUBLIC_AUTH_OAUTH_START_URL, "OAuth 設定時のみ実行");
  const oauthStart = new URL(process.env.PUBLIC_AUTH_OAUTH_START_URL!);
  await page.route(`${oauthStart.origin}${oauthStart.pathname}**`, (route) =>
    route.fulfill({ status: 200, contentType: "text/html", body: "OAuth start reached" }),
  );

  await page.goto("/login/passkey?email=user%40example.test&next=%2Fsettings%2Fsecurity");
  await expect(page).toHaveURL((url) =>
    url.origin === oauthStart.origin &&
    url.pathname === oauthStart.pathname &&
    url.searchParams.get("next") === "/settings/security" &&
    !url.searchParams.has("email"),
  );
});

test("認証が中断されたら自動再試行を止め、利用者が再試行できる", async ({ page }) => {
  test.skip(!process.env.PUBLIC_AUTH_OAUTH_START_URL, "OAuth 設定時のみ実行");
  const oauthStart = new URL(process.env.PUBLIC_AUTH_OAUTH_START_URL!);
  let starts = 0;
  await page.route(`${oauthStart.origin}${oauthStart.pathname}**`, (route) => {
    starts++;
    return route.fulfill({ status: 200, contentType: "text/html", body: "OAuth start reached" });
  });

  await page.goto("/login?auth_error=cancelled&next=%2Fsettings");
  await expect(page.getByRole("heading", { name: "ログインを完了できませんでした" })).toBeVisible();
  expect(starts).toBe(0);
  await page.getByRole("link", { name: "もう一度ログインする" }).click();
  await expect(page).toHaveURL((url) =>
    url.origin === oauthStart.origin && url.pathname === oauthStart.pathname && url.searchParams.get("next") === "/settings",
  );
  expect(starts).toBe(1);
});
