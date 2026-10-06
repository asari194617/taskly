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
