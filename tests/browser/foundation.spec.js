import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

async function openReader(page) {
  await page.goto("/series/last-signal");
  await page.getByRole("link", { name: "Start reading" }).click();
  await expect(page.locator("[data-reader-controls]").first()).toBeVisible();
}
async function blurControls(page) {
  await page.locator("#main h1").click();
}
async function signIn(page) {
  await page.goto("/login");
  await page.getByLabel("Email address").fill("reader@komanook.test");
  await page
    .getByLabel("Password", { exact: true })
    .fill("a quiet place to read");
  await page.getByRole("button", { name: "Sign in", exact: false }).click();
  await expect(page).toHaveURL(/\/account$/);
}

test("catalog search patches results and preserves focus", async ({ page }) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/catalog");
  await expect(page.locator(".series-card")).toHaveCount(3);
  await page.getByLabel("Find a story or creator").fill("Orbit");
  await page.getByLabel("Find a story or creator").press("Enter");
  await expect(page.locator(".series-card")).toHaveCount(1);
  await expect(page.getByRole("heading", { name: "Orbit Post" })).toBeVisible();
  await expect(page.getByLabel("Find a story or creator")).toBeFocused();
  await expect(page).toHaveURL(/\/catalog$/);
  expect(errors).toEqual([]);
});

test("MannerHTML tabs keep selection and keyboard behavior after Datastar updates", async ({
  page,
}) => {
  await page.goto("/series/last-signal");
  const chapters = page.getByRole("tab", { name: "Chapters", exact: true });
  const about = page.getByRole("tab", {
    name: "About this story",
    exact: true,
  });
  await expect(chapters).toHaveAttribute("aria-selected", "true");
  await chapters.focus();
  await chapters.press("ArrowRight");
  await expect(about).toBeFocused();
  await expect(about).toHaveAttribute("aria-selected", "true");
  await page.getByLabel("Find a story or creator").fill("Tides");
  await page.getByLabel("Find a story or creator").press("Enter");
  await expect(page.locator(".series-card")).toHaveCount(1);
  await expect(about).toHaveAttribute("aria-selected", "true");
  await expect(page.getByLabel("Find a story or creator")).toBeFocused();
  await about.focus();
  await about.press("Home");
  await expect(chapters).toBeFocused();
  await expect(chapters).toHaveAttribute("aria-selected", "true");
  await expect(
    page.getByRole("link", { name: /Someone is still listening/ }),
  ).toBeVisible();
});

test("reader supports LTR, RTL, bounded image sources and guest restoration", async ({
  page,
}) => {
  await openReader(page);
  const mode = page.getByLabel("Reading mode");
  const current = page.getByLabel("Current page");
  await mode.selectOption("ltr");
  await blurControls(page);
  await page.keyboard.press("ArrowRight");
  await expect(current).toHaveValue("2");
  await expect(page.locator("[data-reader-page]:not([hidden])")).toHaveCount(1);
  await mode.selectOption("rtl");
  await blurControls(page);
  await page.keyboard.press("ArrowLeft");
  await expect(current).toHaveValue("3");
  await page.keyboard.press("ArrowRight");
  await expect(current).toHaveValue("2");
  await page.keyboard.press("End");
  await expect(current).toHaveValue("4");
  await expect(page.getByRole("button", { name: "Next page" })).toBeDisabled();
  await expect(page.locator("[data-image][src]")).toHaveCount(2);
  await page.keyboard.press("Home");
  await expect(current).toHaveValue("1");
  await current.selectOption("3");
  await page.reload();
  await expect(current).toHaveValue("3");
  await expect(mode).toHaveValue("rtl");
  const chapter = await page
    .locator("[data-reader]")
    .getAttribute("data-chapter");
  const saved = await page.evaluate(
    (key) => JSON.parse(localStorage.getItem("komanook.progress.v1"))[key].page,
    chapter,
  );
  expect(saved).toBe(3);
});

test("continuous scrolling persists the page under the toolbar", async ({
  page,
}) => {
  await openReader(page);
  await page
    .locator("#page-3")
    .evaluate((el) => el.scrollIntoView({ block: "start" }));
  await expect(page.getByLabel("Current page")).toHaveValue("3");
  await page.reload();
  await expect(page.getByLabel("Current page")).toHaveValue("3");
  await expect(page.locator("[data-reader-page]:not([hidden])")).toHaveCount(4);
});

test("reader remains usable when device storage is blocked", async ({
  page,
}) => {
  await page.addInitScript(() => {
    Storage.prototype.getItem = () => {
      throw new Error("blocked");
    };
    Storage.prototype.setItem = () => {
      throw new Error("blocked");
    };
  });
  await openReader(page);
  await page.getByLabel("Reading mode").selectOption("ltr");
  await page.getByRole("button", { name: "Next page" }).click();
  await expect(page.getByLabel("Current page")).toHaveValue("2");
  await expect(page.locator("[data-save-status]")).toContainText(
    "storage unavailable",
  );
});

test("account progress survives a new browser context and logout", async ({
  page,
  browser,
  baseURL,
}) => {
  await signIn(page);
  await openReader(page);
  await page.getByLabel("Reading mode").selectOption("ltr");
  await page.getByLabel("Current page").selectOption("3");
  await expect(page.locator("[data-save-status]")).toHaveText(
    "Saved to your account",
  );
  const nextContext = await browser.newContext({
    baseURL,
    viewport: { width: 390, height: 844 },
  });
  const next = await nextContext.newPage();
  await signIn(next);
  await openReader(next);
  await expect(next.getByLabel("Current page")).toHaveValue("3");
  await nextContext.close();
  await page.goto("/account");
  await page.getByRole("button", { name: "Sign out" }).click();
  await page.goto("/account");
  await expect(page).toHaveURL(/\/login$/);
});

test("failed account sync keeps a local bookmark", async ({ page }) => {
  await signIn(page);
  await page.route("**/api/progress/**", (route) => route.abort());
  await openReader(page);
  await page.getByLabel("Reading mode").selectOption("ltr");
  await page.getByLabel("Current page").selectOption("2");
  await expect(page.locator("[data-save-status]")).toContainText(
    "account sync unavailable",
  );
  await page.reload();
  await expect(page.getByLabel("Current page")).toHaveValue("2");
});

test("native forms, tabs and reader work without JavaScript", async ({
  browser,
  baseURL,
}) => {
  const context = await browser.newContext({
    baseURL,
    javaScriptEnabled: false,
    viewport: { width: 375, height: 812 },
  });
  const page = await context.newPage();
  await page.goto("/catalog");
  await page.getByLabel("Find a story or creator").fill("Orbit");
  await page.getByRole("button", { name: /Explore/ }).click();
  await expect(page.locator(".series-card")).toHaveCount(1);
  await expect(page).toHaveURL(/q=Orbit/);
  await page.goto("/series/last-signal");
  await expect(page.locator("#about")).toBeVisible();
  await expect(page.locator("#chapters")).toBeVisible();
  await page.getByRole("link", { name: "Start reading" }).click();
  await expect(page.locator("[data-reader-page]")).toHaveCount(4);
  await expect(page.locator("[data-reader-controls]").first()).toBeHidden();
  await expect(
    page.getByText("Continuous reading is available without JavaScript.", {
      exact: false,
    }),
  ).toBeVisible();
  await context.close();
});

test("layouts have no horizontal overflow and automated accessibility checks pass", async ({
  page,
}) => {
  for (const path of ["/catalog", "/series/last-signal", "/login"]) {
    await page.goto(path);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    const results = await new AxeBuilder({ page })
      .setLegacyMode(true)
      .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
      .analyze();
    expect(results.violations).toEqual([]);
  }
  await openReader(page);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  const results = await new AxeBuilder({ page })
    .setLegacyMode(true)
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    .analyze();
  expect(results.violations).toEqual([]);
});
