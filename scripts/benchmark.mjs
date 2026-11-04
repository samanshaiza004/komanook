import { chromium } from "@playwright/test";
import { readdir, readFile, writeFile, mkdir } from "node:fs/promises";
import { gzipSync, brotliCompressSync } from "node:zlib";
import { cpus, arch, platform } from "node:os";

const base = process.env.TEST_BASE_URL || "http://localhost:8080";
const browser = await chromium.launch();
await mkdir("docs/screenshots", { recursive: true });
const profiles = [
  { name: "desktop", viewport: { width: 1440, height: 1000 }, cpu: 1 },
  {
    name: "throttled-mobile",
    viewport: { width: 390, height: 844 },
    isMobile: true,
    deviceScaleFactor: 2,
    hasTouch: true,
    cpu: 4,
  },
];
const results = [];
try {
  for (const profile of profiles) {
    for (const route of ["/catalog", "/read/1"]) {
      const runs = [];
      for (let iteration = 0; iteration < 3; iteration++) {
        const { name, cpu, ...options } = profile;
        const context = await browser.newContext({
          ...options,
          serviceWorkers: "block",
        });
        const page = await context.newPage();
        const cdp = await context.newCDPSession(page);
        await cdp.send("Network.enable");
        await cdp.send("Network.setCacheDisabled", { cacheDisabled: true });
        await cdp.send("Emulation.setCPUThrottlingRate", { rate: cpu });
        if (cpu > 1)
          await cdp.send("Network.emulateNetworkConditions", {
            offline: false,
            latency: 150,
            downloadThroughput: 200000,
            uploadThroughput: 75000,
          });
        await page.addInitScript(() => {
          window.__bench = { lcp: 0, cls: 0 };
          new PerformanceObserver((list) => {
            for (const entry of list.getEntries())
              window.__bench.lcp = entry.startTime;
          }).observe({ type: "largest-contentful-paint", buffered: true });
          new PerformanceObserver((list) => {
            for (const entry of list.getEntries())
              if (!entry.hadRecentInput) window.__bench.cls += entry.value;
          }).observe({ type: "layout-shift", buffered: true });
        });
        await page.goto(base + route, { waitUntil: "networkidle" });
        await page.waitForTimeout(300);
        const metrics = await page.evaluate(() => {
          const nav = performance.getEntriesByType("navigation")[0];
          const resources = performance.getEntriesByType("resource");
          return {
            ttfbMs: nav.responseStart,
            lcpMs: window.__bench.lcp,
            domContentLoadedMs: nav.domContentLoadedEventEnd,
            loadMs: nav.loadEventEnd,
            cls: window.__bench.cls,
            requests: resources.length + 1,
            transferBytes: resources.reduce(
              (sum, r) => sum + r.transferSize,
              nav.transferSize,
            ),
            images: resources
              .filter((r) => r.initiatorType === "img")
              .map((r) => ({
                path: new URL(r.name).pathname.split("/").pop(),
                durationMs: r.duration,
                bytes: r.encodedBodySize,
              })),
            loadedImages: [...document.images].filter(
              (i) => i.complete && i.naturalWidth > 0,
            ).length,
          };
        });
        if (iteration === 0)
          await page.screenshot({
            path: `docs/screenshots/${name}-${route.includes("read") ? "reader" : "catalog"}.png`,
            fullPage: route === "/catalog",
          });
        if (route.startsWith("/read")) {
          await page.getByLabel("Reading mode").selectOption("ltr");
          await cdp.send("HeapProfiler.collectGarbage");
          const before = await cdp.send("Runtime.getHeapUsage");
          for (let turn = 0; turn < 40; turn++)
            await page
              .getByLabel("Current page")
              .selectOption(String((turn % 4) + 1));
          await cdp.send("HeapProfiler.collectGarbage");
          const after = await cdp.send("Runtime.getHeapUsage");
          metrics.reader = {
            turns: 40,
            jsHeapBeforeBytes: before.usedSize,
            jsHeapAfterBytes: after.usedSize,
            jsHeapGrowthBytes: after.usedSize - before.usedSize,
            dom: await cdp.send("Memory.getDOMCounters"),
            imageSources: await page.locator("[data-image][src]").count(),
          };
        }
        runs.push(metrics);
        await context.close();
      }
      const median = (key) => runs.map((r) => r[key]).sort((a, b) => a - b)[1];
      results.push({
        profile: profile.name,
        route,
        median: {
          ttfbMs: median("ttfbMs"),
          lcpMs: median("lcpMs"),
          loadMs: median("loadMs"),
          cls: median("cls"),
          transferBytes: median("transferBytes"),
          requests: median("requests"),
        },
        runs,
      });
    }
  }
  const assets = [];
  async function walk(directory) {
    for (const item of await readdir(directory, { withFileTypes: true })) {
      const file = `${directory}/${item.name}`;
      if (item.isDirectory()) await walk(file);
      else if (/\.(js|css)$/.test(file)) {
        const body = await readFile(file);
        assets.push({
          path: file.replace("internal/web/static/", ""),
          bytes: body.length,
          gzipBytes: gzipSync(body).length,
          brotliBytes: brotliCompressSync(body).length,
        });
      }
    }
  }
  await walk("internal/web/static");
  const report = {
    date: "2026-11-03",
    environment: {
      platform: platform(),
      architecture: arch(),
      cpu: cpus()[0].model,
      logicalCPUs: cpus().length,
      node: process.version,
      chromium: browser.version(),
    },
    method: {
      runs: 3,
      cache: "Cold new context; HTTP cache disabled",
      mobile:
        "390×844 CSS px, DPR 2, 4× CPU slowdown, 150 ms network latency, 200,000 B/s down, 75,000 B/s up",
      origin:
        "Local HTTP server and real local PostgreSQL; no CDN, TLS or response compression",
      reader:
        "Four original 1000×1400 WebP pages; 40 page selections; JS heap sampled after forced GC",
    },
    assets,
    results,
  };
  await writeFile(
    "docs/performance-results.json",
    JSON.stringify(report, null, 2) + "\n",
  );
  const rows = results
    .map(
      (r) =>
        `| ${r.profile} | ${r.route} | ${r.median.ttfbMs.toFixed(0)} | ${r.median.lcpMs.toFixed(0)} | ${r.median.loadMs.toFixed(0)} | ${(r.median.transferBytes / 1024).toFixed(1)} | ${r.median.requests} | ${r.median.cls.toFixed(3)} |`,
    )
    .join("\n");
  const bundleRows = assets
    .map(
      (a) =>
        `| ${a.path} | ${(a.bytes / 1024).toFixed(1)} | ${(a.gzipBytes / 1024).toFixed(1)} | ${(a.brotliBytes / 1024).toFixed(1)} |`,
    )
    .join("\n");
  const memoryRows = results
    .filter((r) => r.route.startsWith("/read"))
    .map((r) => {
      const runs = r.runs.map((r) => r.reader);
      return `| ${r.profile} | ${Math.min(...runs.map((r) => r.jsHeapGrowthBytes))}–${Math.max(...runs.map((r) => r.jsHeapGrowthBytes))} | ${runs.map((r) => r.imageSources).join(", ")} |`;
    })
    .join("\n");
  await writeFile(
    "docs/performance.md",
    `# Foundation measurements\n\nMeasured 2026-11-03 using Chromium ${browser.version()} on ${report.environment.cpu} (${platform()}/${arch()}). Raw observations: [performance-results.json](performance-results.json). Reproduce with a running seeded application: \`npm run bench\`, or run \`npm run test:local\` for the complete isolated check.\n\n## Cold navigation\n\nThree fresh contexts per workflow; table reports medians, not percentiles or field data. Desktop: 1440×1000, unthrottled. Mobile: ${report.method.mobile}. Navigation waits for network idle and another 300 ms for observation. LCP is the last observed candidate at that point, not an end-of-session field metric.\n\n| Profile | Route | TTFB ms | LCP ms | Load ms | Transfer KiB | Requests | CLS |\n| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |\n${rows}\n\nThe origin is local HTTP with a real local PostgreSQL database. There is no CDN, TLS or HTTP response compression; transfer sizes include response headers and only images actually requested in the initial viewport/lazy-loading margin. Production latency, compression, server hardware and cache hit rate will change these numbers. There is no benchmark against another platform and no performance-superiority claim.\n\n## Browser assets\n\n| Asset | Raw KiB | Gzip KiB | Brotli KiB |\n| --- | ---: | ---: | ---: |\n${bundleRows}\n\nCompressed sizes are calculated artifacts, not observed local wire sizes. Catalog loads Datastar and CSS. Series additionally loads the MannerHTML tabs module and its two dependencies. Reader loads CSS and its own module, with no Datastar or MannerHTML runtime. Account pages load CSS only. There are no remote font or analytics requests.\n\n## Reader memory and images\n\nFour original WebP fixture pages, each 1000×1400. After switching to pagination, sample JS heap after forced GC, select a page 40 times, force GC again, then sample.\n\n| Profile | JS heap growth across three runs, bytes | Image sources retained in each run |\n| --- | ---: | --- |\n${memoryRows}\n\nA source window of the current page plus one neighbor on each side is enforced in pagination. DOM nodes for all pages remain so the no-JS document and semantics stay simple. Continuous mode uses native lazy loading and keeps loaded sources. These heap samples do **not** measure native decoded-image/GPU memory, prove leak freedom, or validate a 200-page chapter on a low-memory phone. The four-page fixtures are intentionally small. A real licensed long chapter, decoded-memory profiling, adaptive resolutions and mobile assistive-technology checks are release gates before a large public catalog. Per-image request durations and bytes are retained in the raw report.\n\n## Budgets for this fixture\n\nAim for reader custom JS below 12 KiB raw, catalog initial transfer below 350 KiB, and zero unexpected layout shift. Browser tests assert source-window bounds, keyboard operation, persistence and narrow-screen overflow. These are regression targets for this slice, not production promises. Screenshots in [screenshots](screenshots/) were inspected at desktop and mobile widths. Automated axe checks supplement browser tests; they do not certify accessibility.\n`,
  );
  console.log(
    JSON.stringify(
      results.map(({ profile, route, median }) => ({
        profile,
        route,
        ...median,
      })),
      null,
      2,
    ),
  );
} finally {
  await browser.close();
}
