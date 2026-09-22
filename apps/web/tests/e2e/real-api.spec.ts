import { execFileSync } from "node:child_process";
import { expect, test, type Page } from "@playwright/test";

const api = "http://localhost:3063";
test.setTimeout(90_000);

test.beforeEach(() => {
  execFileSync(process.env.E2E_SEED_BIN || "../server/e2e-seed", [], { stdio: "inherit", env: process.env });
});

async function login(page: Page, email: string) {
  await page.addInitScript(() => localStorage.setItem("display_lang", "ja"));
  await page.goto("/login");
  await expect(page.locator('astro-island[component-url*="Begin"]')).not.toHaveAttribute("ssr", "");
  await page.getByLabel("メールアドレス").fill(email);
  await page.getByRole("button", { name: "次へ" }).click();
  await expect(page).toHaveURL(/\/login\/password/);
  await expect(page.locator('astro-island[component-url*="Password"]')).not.toHaveAttribute("ssr", "");
  await page.getByLabel("パスワード").fill("e2e-password");
  await page.getByRole("button", { name: "次へ" }).click();
  await expect(page).toHaveURL(/\/consent\?next=%2Fhome$/);
  const policyCheckboxes = page.getByRole("checkbox");
  const acceptPolicies = page.getByRole("button", { name: "同意して進む" });
  await expect(policyCheckboxes).toHaveCount(3);
  await expect(acceptPolicies).toBeDisabled();
  for (let i = 0; i < 3; i++) {
    await policyCheckboxes.nth(i).click();
    await expect(policyCheckboxes.nth(i)).toBeChecked();
  }
  await expect(acceptPolicies).toBeEnabled();
  await acceptPolicies.click();
  await expect(page).toHaveURL(/\/home$/);
  await expect(page.locator('astro-island[component-url*="TasksList"]')).not.toHaveAttribute("ssr", "");
  await expect(page.getByRole("button", { name: "新規作成" }).first()).toBeVisible();
}

test("実 API と DB でログイン・課題の作成・一覧・編集・完了・削除", async ({ page }) => {
  await login(page, "e2e-admin@example.test");
  await page.getByRole("button", { name: "新規作成" }).first().click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("タイトル").fill("E2E 課題");
  await dialog.getByRole("button", { name: "選択してください" }).first().click();
  await page.getByRole("menuitem", { name: "数学" }).click();
  await dialog.getByRole("button", { name: "未設定" }).click();
  await page.getByRole("button", { name: "日付", exact: true }).click();
  await page.locator(".rdp-today button").click();
  await page.getByRole("button", { name: "適用" }).click();
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(page.getByText("E2E 課題")).toBeVisible();
  await page.reload();
  await expect(page.getByText("E2E 課題")).toBeVisible();

  await page.getByRole("button", { name: "アクション" }).click();
  await page.getByRole("button", { name: "編集" }).click();
  await page.getByLabel("タイトル").last().fill("E2E 編集済み課題");
  await page.getByRole("button", { name: "保存" }).click();
  await expect(page.getByText("E2E 編集済み課題")).toBeVisible();
  const completion = page.waitForResponse((response) => response.url() === `${api}/graphql` && !!response.request().postData()?.includes("completeTask"));
  await page.getByRole("checkbox", { name: "E2E 編集済み課題: 完了" }).click();
  expect((await (await completion).json()).data.completeTask).toBe(true);
  await expect(page.getByRole("checkbox", { name: "E2E 編集済み課題: 完了" })).toBeChecked();
  await page.reload();
  await expect(page.getByRole("checkbox", { name: "E2E 編集済み課題: 完了" })).toBeChecked();

  await page.getByRole("button", { name: "アクション" }).click();
  await page.getByRole("button", { name: "削除" }).click();
  const deletion = page.waitForResponse((response) => response.url() === `${api}/graphql` && !!response.request().postData()?.includes("deleteTask"));
  await page.getByRole("dialog", { name: "削除の確認" }).getByRole("button", { name: "削除する" }).click();
  expect((await (await deletion).json()).data.deleteTask).toBe(true);
  await expect(page.getByText("E2E 編集済み課題")).toHaveCount(0);
  await page.reload();
  await expect(page.getByText("E2E 編集済み課題")).toHaveCount(0);
});

test("通常ユーザーの管理操作は実 API で拒否される", async ({ page }) => {
  await login(page, "e2e-user@example.test");
  const result = await page.evaluate(async (url) => {
    const response = await fetch(`${url}/graphql`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json", "X-Client-Platform": "web" },
      body: JSON.stringify({ query: 'mutation { adminSubjectCreate(name_ja: "不正な教科") { id } }' }),
    });
    return { status: response.status, body: await response.json() };
  }, api);
  expect(result.status).toBe(400);
  expect(JSON.stringify(result.body)).toContain("forbidden");
  const subjects = await page.evaluate(async (url) => {
    const response = await fetch(`${url}/graphql`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json", "X-Client-Platform": "web" },
      body: JSON.stringify({ query: "query { subjects { name_ja } }" }),
    });
    return response.json();
  }, api);
  expect(subjects.data.subjects).not.toContainEqual({ name_ja: "不正な教科" });
});
