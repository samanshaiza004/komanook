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

test("MangaDex quick searches keep the three requested titles discoverable", async ({
  page,
}) => {
  await page.goto("/catalog");
  const searches = page.getByRole("navigation", {
    name: "Popular MangaDex searches",
  });
  await expect(
    searches.getByRole("link", { name: "Demon Slayer" }),
  ).toHaveAttribute("href", "/mangadex?q=Demon%20Slayer");
  await expect(
    searches.getByRole("link", { name: "Jujutsu Kaisen" }),
  ).toHaveAttribute("href", "/mangadex?q=Jujutsu%20Kaisen");
  await expect(
    searches.getByRole("link", { name: "Kiichi!!" }),
  ).toHaveAttribute("href", "/mangadex?q=Kiichi%21%21");
  await expect(
    page.getByText("While MangaDex is available", { exact: false }),
  ).toBeVisible();
});

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
    (key) =>
      JSON.parse(localStorage.getItem("komanook.progress.guest.v1"))[key].page,
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

test("guest progress stays separate and imports without replacing account bookmarks", async ({
  page,
}, testInfo) => {
  const extraSeries =
    testInfo.project.name === "mobile" ? "orbit-post" : "between-tides";
  await signIn(page);
  await openReader(page);
  const firstChapter = await page
    .locator("[data-reader]")
    .getAttribute("data-chapter");
  await page.getByLabel("Reading mode").selectOption("ltr");
  await page.getByLabel("Current page").selectOption("1");
  await expect(page.locator("[data-save-status]")).toHaveText(
    "Saved to your account",
  );
  const accountKey = await page
    .locator("[data-reader]")
    .getAttribute("data-account-key");

  await page.goto("/account");
  await page.getByRole("button", { name: "Sign out" }).click();
  await page.evaluate(() =>
    localStorage.removeItem("komanook.progress.guest.v1"),
  );
  await openReader(page);
  await page.getByLabel("Reading mode").selectOption("ltr");
  await page.getByLabel("Current page").selectOption("2");
  await page.goto(`/series/${extraSeries}`);
  await page.getByRole("link", { name: "Start reading" }).click();
  await expect(page.locator("[data-reader-controls]").first()).toBeVisible();
  const secondChapter = await page
    .locator("[data-reader]")
    .getAttribute("data-chapter");
  await page.getByLabel("Reading mode").selectOption("ltr");
  await page.getByLabel("Current page").selectOption("3");
  await expect
    .poll(() =>
      page.evaluate(
        ({ key, chapters }) => {
          const entries = JSON.parse(localStorage.getItem(key) || "{}");
          return chapters.map((chapter) => entries[chapter]?.page);
        },
        {
          key: "komanook.progress.guest.v1",
          chapters: [firstChapter, secondChapter],
        },
      ),
    )
    .toEqual([2, 3]);

  await signIn(page);
  await openReader(page);
  await expect(page.getByLabel("Current page")).toHaveValue("1");
  const accountStorageKey = `komanook.progress.account.${accountKey}.v1`;
  const namespaces = await page.evaluate(
    ([guestKey, accountKey]) => ({
      guest: JSON.parse(localStorage.getItem(guestKey) || "{}"),
      account: JSON.parse(localStorage.getItem(accountKey) || "{}"),
    }),
    ["komanook.progress.guest.v1", accountStorageKey],
  );
  expect(namespaces.guest[firstChapter].page).toBe(2);
  expect(namespaces.account[firstChapter].page).toBe(1);
  await page.goto("/account");
  await page
    .getByRole("button", { name: "Import guest reading places" })
    .click();
  await expect(page.locator("[data-import-status]")).toHaveText(
    "1 imported; 1 already saved in your account.",
  );
  await expect(
    page.getByRole("button", { name: "Guest reading places imported" }),
  ).toBeFocused();
  expect(
    await page.evaluate(() =>
      localStorage.getItem("komanook.progress.guest.v1"),
    ),
  ).toBeNull();

  await page.goto(`/read/${secondChapter}`);
  await expect(page.getByLabel("Current page")).toHaveValue("3");
  await expect(page.locator("[data-reader]")).toHaveAttribute(
    "data-chapter",
    secondChapter,
  );
  await page.goto(`/api/progress/${firstChapter}`);
  await expect(page.locator("body")).toContainText('"page":1');
});

test("long chapters keep paginated image sources bounded", async ({ page }) => {
  await page.goto("/read/1");
  const readerScript = await page
    .locator('script[src*="reader.js"]')
    .getAttribute("src");
  const imageSource = await page
    .locator("[data-image]")
    .first()
    .getAttribute("data-src");
  await page.goto("/catalog");
  await page.evaluate((src) => {
    document.body.replaceChildren();
    const reader = document.createElement("main");
    reader.dataset.reader = "";
    reader.dataset.chapter = "long-fixture";
    reader.dataset.csrf = "";
    reader.dataset.signedIn = "false";
    reader.innerHTML = `
      <h1 id="long-title">Long chapter fixture</h1>
      <div class="reader-toolbar" data-reader-controls hidden>
        <label for="long-mode">Reading mode</label>
        <select id="long-mode" data-mode>
          <option value="vertical">Vertical scroll</option>
          <option value="ltr">Pages, left to right</option>
          <option value="rtl">Pages, right to left</option>
        </select>
        <button type="button" data-previous aria-label="Previous page">←</button>
        <label for="long-page">Current page</label>
        <select id="long-page" data-page></select>
        <button type="button" data-next aria-label="Next page">→</button>
        <p data-save-status role="status"></p>
      </div>
      <p data-reader-fallback>Continuous reading is available without JavaScript.</p>
      <div data-pages></div>
      <p data-page-announcement aria-live="polite"></p>`;
    const pages = reader.querySelector("[data-pages]");
    const select = reader.querySelector("[data-page]");
    for (let number = 1; number <= 250; number++) {
      const option = document.createElement("option");
      option.value = String(number);
      option.textContent = `Page ${number} / 250`;
      select.append(option);
      const figure = document.createElement("figure");
      figure.dataset.readerPage = "";
      const image = document.createElement("img");
      image.dataset.image = "";
      image.dataset.src = src;
      image.width = 1000;
      image.height = 1400;
      image.alt = `Long fixture page ${number}`;
      figure.append(image);
      pages.append(figure);
    }
    document.body.append(reader);
  }, imageSource);
  await page.addScriptTag({
    url: new URL(readerScript, page.url()).toString(),
  });
  await expect(page.locator("[data-reader-controls]")).toBeVisible();
  await expect(page.locator("[data-reader-page]")).toHaveCount(250);
  await page.getByLabel("Reading mode").selectOption("ltr");
  await page.getByLabel("Current page").selectOption("125");
  await expect(page.locator("[data-image][src]")).toHaveCount(3);
  await expect(page.locator("[data-reader-page]")).toHaveCount(250);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.locator("#long-title").click();
  await page.keyboard.press("End");
  await expect(page.getByLabel("Current page")).toHaveValue("250");
  await expect(page.locator("[data-image][src]")).toHaveCount(2);
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
