import { expect, test, type Page } from "@playwright/test";

async function setup(page: Page, count = 12) {
  const state = {
    groups: Array.from({ length: count }, (_, i) => ({
      id: i + 1,
      cycle_id: 7,
      position: i,
      user_ids: [i + 1],
    })),
    cycles: [
      {
        id: 7,
        description: "2学期",
        begin_date: "2026-09-01",
        finish_date: "2026-12-31",
      },
    ],
    index: 10,
    mutations: [] as string[],
    failGroup: false,
  };
  const users = Array.from({ length: Math.max(15, count) }, (_, i) => ({
    id: i + 1,
    display_name: `メンバー${i + 1}`,
    email: `user${i + 1}@example.test`,
  }));
  await page.route("http://localhost:3063/**", async (route) => {
    const { query: q, variables: v = {} } = route.request().postDataJSON();
    const respond = (data: unknown) =>
      route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({ data }),
      });
    if (q.includes("me{"))
      return respond({
        me: {
          id: 1,
          email: "admin@example.test",
          display_name: "管理者",
          is_admin: true,
          is_manager: false,
          is_agreed_policies: true,
        },
      });
    if (q.includes("adminDayshiftCyclesPaginated"))
      return respond({
        adminDayshiftCyclesPaginated: {
          items: state.cycles,
          total: state.cycles.length,
        },
        dayShiftCurrentCycle: 7,
      });
    if (q.includes("adminDayshiftGroupsPaginated")) {
      // Match the server cost for items and its nested user_ids list.
      if (2 + v.size * 24 > 1000)
        return route.fulfill({
          status: 400,
          contentType: "application/json",
          body: JSON.stringify({ error: "query too complex" }),
        });
      const groups = state.groups.filter((g) => g.cycle_id === v.cid);
      return respond({
        adminDayshiftGroupsPaginated: {
          items: groups.slice((v.page - 1) * v.size, v.page * v.size),
          total: groups.length,
        },
      });
    }
    if (q.includes("adminUsersByIds"))
      return respond({
        adminUsersByIds: users.filter((u) => v.ids.includes(u.id)),
      });
    if (q.includes("adminUsers("))
      return respond({
        adminUsers: users.filter((u) => !v.q || u.display_name.includes(v.q)),
      });
    if (q.includes("mutation")) state.mutations.push(q);
    if (q.includes("adminDayshiftCreateGroup")) {
      if (state.failGroup)
        return route.fulfill({
          contentType: "application/json",
          body: JSON.stringify({ errors: [{ message: "保存エラー" }] }),
        });
      const group = {
        id: 100 + state.groups.length,
        cycle_id: v.cid,
        position: state.groups.length,
        user_ids: v.ids,
      };
      state.groups.push(group);
      return respond({ adminDayshiftCreateGroup: { id: group.id } });
    }
    if (q.includes("adminDayshiftUpdateGroupMembers")) {
      state.groups.find((g) => g.id === v.gid)!.user_ids = v.ids;
      return respond({ adminDayshiftUpdateGroupMembers: true });
    }
    if (q.includes("adminDayshiftReorderGroups")) {
      state.groups.sort((a, b) => v.ids.indexOf(a.id) - v.ids.indexOf(b.id));
      state.groups.forEach((g, i) => (g.position = i));
      return respond({ adminDayshiftReorderGroups: true });
    }
    if (q.includes("adminDayshiftDeleteGroup")) {
      state.groups = state.groups.filter((g) => g.id !== v.gid);
      return respond({ adminDayshiftDeleteGroup: true });
    }
    if (q.includes("adminDayshiftCreateCycle")) {
      state.cycles.push({
        id: 8,
        description: v.d,
        begin_date: v.b,
        finish_date: v.f,
      });
      return respond({ adminDayshiftCreateCycle: { id: 8 } });
    }
    if (q.includes("adminUpdateDayShiftIndex")) {
      state.index = v.v;
      return respond({ adminUpdateDayShiftIndex: true });
    }
    if (q.includes("dayShiftCurrentCycle"))
      return respond({ dayShiftCurrentCycle: 7 });
    if (q.includes("dayShiftIndex"))
      return respond({ dayShiftIndex: state.index });
    throw new Error(`Unexpected operation ${q}`);
  });
  await page.goto("/admin/day-shift/");
  await expect(
    page.getByRole("button", { name: "グループを追加", exact: true }),
  ).toBeEnabled();
  return state;
}

test("自動選択した期間で追加・編集・並べ替え・削除ができ、全グループで担当順を表示する", async ({
  page,
}) => {
  const state = await setup(page);
  await page.screenshot({
    path: "/tmp/taskly-day-shift-desktop.png",
    fullPage: true,
  });
  const preview = page.getByRole("complementary").last();
  await expect(preview.getByRole("listitem").first()).toContainText(
    "メンバー11",
  );
  await expect(preview.getByRole("listitem").nth(2)).toContainText("メンバー1");
  await page
    .getByRole("button", { name: "グループを追加", exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "メンバーを検索" })
    .fill("メンバー15");
  await page.getByRole("checkbox", { name: "メンバー15" }).check();
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect.poll(() => state.groups.length).toBe(13);
  await page
    .getByRole("button", { name: "1番のグループを編集", exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "メンバーを検索" })
    .fill("メンバー14");
  await page.getByRole("checkbox", { name: "メンバー14" }).check();
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect.poll(() => state.groups[0].user_ids).toEqual([1, 14]);
  await page
    .getByRole("button", { name: "1番のグループを下へ", exact: true })
    .click();
  await expect.poll(() => state.groups[0].id).toBe(2);
  await page
    .getByRole("button", { name: "1番のグループを削除", exact: true })
    .click();
  await page.getByRole("button", { name: "キャンセル", exact: true }).click();
  expect(state.groups).toHaveLength(13);
  await page
    .getByRole("button", { name: "1番のグループを削除", exact: true })
    .click();
  await page.getByRole("button", { name: "実行する" }).click();
  await expect.poll(() => state.groups.length).toBe(12);
});

test("期間作成の日付検証とモバイル表示", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const state = await setup(page, 0);
  await page.getByRole("button", { name: "期間を作成", exact: true }).click();
  await page.getByLabel("期間名", { exact: true }).fill("3学期");
  await page.getByLabel("開始日", { exact: true }).fill("2027-01-08");
  await page.getByLabel("終了日", { exact: true }).fill("2027-01-01");
  await expect(
    page.getByRole("button", { name: "保存", exact: true }),
  ).toBeDisabled();
  await page.getByLabel("終了日", { exact: true }).fill("2027-03-31");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByLabel("管理する期間")).toHaveValue("8");
  await expect(
    page.getByRole("button", { name: "グループを追加", exact: true }),
  ).toBeEnabled();
  expect(state.cycles).toHaveLength(2);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "/tmp/taskly-day-shift-mobile.png",
    fullPage: true,
  });
});

test("保存失敗時は編集内容を保持し、CSVの不正なIDを送信しない", async ({
  page,
}) => {
  const state = await setup(page);
  await page
    .getByRole("button", { name: "グループを追加", exact: true })
    .click();
  await page.getByRole("checkbox", { name: "メンバー15" }).check();
  state.failGroup = true;
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText(
    "保存エラー",
  );
  await expect(
    page.getByRole("checkbox", { name: "メンバー15" }),
  ).toBeChecked();
  await page.getByRole("button", { name: "キャンセル", exact: true }).click();
  await page.getByRole("button", { name: "CSVでまとめて追加" }).click();
  await page.getByLabel("グループのCSV").fill("1,2\n3oops,4");
  const mutations = state.mutations.length;
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText(
    "2行目",
  );
  expect(state.mutations).toHaveLength(mutations);
});

test("担当位置の変更は確認後に保存する", async ({ page }) => {
  const state = await setup(page);
  await page.getByText("担当位置の調整", { exact: true }).click();
  await page.getByLabel("担当位置（0から）").fill("-1");
  await expect(
    page.getByRole("button", { name: "担当位置を保存" }),
  ).toBeDisabled();
  await page.getByLabel("担当位置（0から）").fill("11");
  await page.getByRole("button", { name: "担当位置を保存" }).click();
  expect(state.index).toBe(10);
  await page.getByRole("button", { name: "実行する" }).click();
  await expect.poll(() => state.index).toBe(11);
  await expect(
    page.getByRole("complementary").last().getByRole("listitem").first(),
  ).toContainText("メンバー12");
});

test("負荷制限内で複数ページのグループとメンバーをすべて読み込む", async ({ page }) => {
  await setup(page, 65);
  await expect(
    page.getByRole("button", { name: /番のグループを編集$/ }),
  ).toHaveCount(65);
  await expect(
    page.getByRole("button", { name: "65番のグループを編集", exact: true }),
  ).toBeEnabled();
  await expect(page.getByText("メンバー65", { exact: true }).first()).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
});
