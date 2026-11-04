const importRoot = document.querySelector("[data-progress-import]");
if (importRoot) initializeProgressImport(importRoot);

function initializeProgressImport(root) {
  const storageKey = "komanook.progress.guest.v1";
  const button = root.querySelector("[data-import-progress]");
  const count = root.querySelector("[data-progress-count]");
  const status = root.querySelector("[data-import-status]");
  let imported = false;

  function readGuestProgress() {
    const text = localStorage.getItem(storageKey);
    if (!text || text.length > 30000) return { text, data: {} };
    const data = JSON.parse(text);
    return {
      text,
      data:
        data && typeof data === "object" && !Array.isArray(data) ? data : {},
    };
  }

  let original;
  try {
    original = readGuestProgress();
  } catch {
    return;
  }
  const entries = Object.entries(original.data)
    .filter(
      ([chapter, value]) =>
        /^[1-9]\d{0,17}$/.test(chapter) &&
        value &&
        Number.isInteger(value.page) &&
        value.page > 0 &&
        Number.isFinite(value.at),
    )
    .sort((a, b) => b[1].at - a[1].at)
    .slice(0, 99);
  if (!entries.length) return;

  count.textContent = `${entries.length} saved ${entries.length === 1 ? "chapter" : "chapters"}`;
  root.hidden = false;
  button.addEventListener("click", async () => {
    if (imported) return;
    button.disabled = true;
    status.textContent = "Importing your saved places…";
    try {
      const response = await fetch("/api/progress/import", {
        method: "POST",
        credentials: "same-origin",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": root.dataset.csrf,
        },
        body: JSON.stringify({
          items: entries.map(([chapter, value]) => ({
            chapter: Number(chapter),
            page: value.page,
          })),
        }),
        signal: AbortSignal.timeout(15000),
      });
      if (!response.ok) throw new Error("import unavailable");
      const result = await response.json();
      if (localStorage.getItem(storageKey) === original.text)
        localStorage.removeItem(storageKey);
      status.textContent = `${result.imported} imported; ${result.existing} already saved in your account.`;
      imported = true;
      button.textContent = "Guest reading places imported";
      button.setAttribute("aria-disabled", "true");
      button.disabled = false;
      button.focus();
    } catch {
      status.textContent =
        "We couldn’t import these places. They’re still saved on this device; try again.";
      button.disabled = false;
      button.focus();
    }
  });
}
