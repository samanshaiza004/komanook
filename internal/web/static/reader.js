// Independent reader: browser APIs + a small same-origin progress contract.
const root = document.querySelector("[data-reader]");
if (root) void startReader(root);

async function startReader(root) {
  const figures = [...root.querySelectorAll("[data-reader-page]")];
  if (!figures.length) return;
  const images = figures.map((figure) => figure.querySelector("[data-image]"));
  const modeSelect = root.querySelector("[data-mode]");
  const pageSelect = root.querySelector("[data-page]");
  const previous = root.querySelector("[data-previous]");
  const next = root.querySelector("[data-next]");
  const status = root.querySelector("[data-save-status]");
  const announcement = root.querySelector("[data-page-announcement]");
  const strip = root.querySelector("[data-pages]");
  const chapter = root.dataset.chapter;
  const endpoint = `/api/progress/${encodeURIComponent(chapter)}`;
  const signedIn = root.dataset.signedIn === "true";
  const guestStorageKey = "komanook.progress.guest.v1";
  const storageKey = signedIn
    ? `komanook.progress.account.${root.dataset.accountKey}.v1`
    : guestStorageKey;
  const modeKey = "komanook.reader-mode.v1";
  let current = 0;
  let mode = "vertical";
  let saving = false;
  let pendingPage = null;
  let saveTimer;
  let scrollFrame = 0;
  let localAvailable = true;

  function bookmarks() {
    try {
      const text = localStorage.getItem(storageKey);
      if (text && text.length > 30000) return {};
      const value = JSON.parse(text || "{}");
      return value && typeof value === "object" && !Array.isArray(value)
        ? value
        : {};
    } catch {
      localAvailable = false;
      return {};
    }
  }
  function validPage(page) {
    return Number.isInteger(page) && page >= 1 && page <= figures.length;
  }
  function saveLocal(page) {
    try {
      const entries = Object.entries(bookmarks()).filter(
        ([key, value]) =>
          key !== chapter &&
          value &&
          Number.isInteger(value.page) &&
          Number.isFinite(value.at),
      );
      entries.sort((a, b) => b[1].at - a[1].at);
      entries.unshift([chapter, { page, at: Date.now() }]);
      const data = Object.fromEntries(entries.slice(0, 99));
      localStorage.setItem(storageKey, JSON.stringify(data));
      localAvailable = true;
    } catch {
      localAvailable = false;
    }
  }
  function localMessage() {
    return localAvailable
      ? "Saved on this device"
      : "Device storage unavailable";
  }

  // Serialize writes so a slow older request cannot overtake the latest bookmark.
  async function flush() {
    if (!signedIn || saving || pendingPage === null) return;
    saving = true;
    const page = pendingPage;
    pendingPage = null;
    try {
      const response = await fetch(endpoint, {
        method: "POST",
        credentials: "same-origin",
        keepalive: true,
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": root.dataset.csrf,
        },
        body: JSON.stringify({ page }),
        signal: AbortSignal.timeout(8000),
      });
      if (!response.ok) throw new Error("progress unavailable");
      status.textContent = "Saved to your account";
    } catch {
      status.textContent = localMessage() + " · account sync unavailable";
    } finally {
      saving = false;
      if (pendingPage !== null) void flush();
    }
  }
  function persist() {
    const page = current + 1;
    saveLocal(page);
    status.textContent = localMessage();
    if (signedIn) {
      pendingPage = page;
      clearTimeout(saveTimer);
      saveTimer = setTimeout(() => void flush(), 350);
    }
  }

  function render({ scroll = false, save = true, announce = true } = {}) {
    pageSelect.value = String(current + 1);
    previous.disabled = current === 0;
    next.disabled = current === figures.length - 1;
    previous.textContent = mode === "rtl" ? "→" : "←";
    next.textContent = mode === "rtl" ? "←" : "→";
    strip.dataset.mode = mode;
    figures.forEach((figure, index) => {
      const visible = mode === "vertical" || index === current;
      figure.hidden = !visible;
      const load = mode === "vertical" || Math.abs(index - current) <= 1;
      if (load) {
        images[index].loading =
          mode === "vertical" && index !== current ? "lazy" : "eager";
        if (!images[index].hasAttribute("src"))
          images[index].src = images[index].dataset.src;
      } else images[index].removeAttribute("src");
    });
    if (scroll) {
      figures[current].scrollIntoView({ block: "start", behavior: "instant" });
    }
    if (announce)
      announcement.textContent = `Page ${current + 1} of ${figures.length}`;
    if (save) persist();
  }
  function go(index) {
    if (index < 0 || index >= figures.length) return;
    current = index;
    render({ scroll: true });
  }

  const local = bookmarks()[chapter];
  let restoredPage = validPage(local?.page) ? local.page : 1;
  try {
    const savedMode = localStorage.getItem(modeKey);
    if (["vertical", "ltr", "rtl"].includes(savedMode)) mode = savedMode;
  } catch {
    /* preferences are optional */
  }
  if (signedIn) {
    try {
      const response = await fetch(endpoint, {
        credentials: "same-origin",
        signal: AbortSignal.timeout(3000),
      });
      if (!response.ok) throw new Error("progress unavailable");
      const remote = await response.json();
      if (validPage(remote.page)) restoredPage = remote.page;
      status.textContent = "Your account keeps your place";
    } catch {
      status.textContent = localMessage() + " · account sync unavailable";
    }
  }
  current = restoredPage - 1;
  modeSelect.value = mode;
  root.querySelectorAll("[data-reader-controls]").forEach((el) => {
    el.hidden = false;
  });
  root.querySelector("[data-reader-fallback]").hidden = true;
  render({ save: false, announce: false, scroll: current > 0 });

  previous.addEventListener("click", () => go(current - 1));
  next.addEventListener("click", () => go(current + 1));
  pageSelect.addEventListener("change", () => go(Number(pageSelect.value) - 1));
  modeSelect.addEventListener("change", () => {
    mode = modeSelect.value;
    try {
      localStorage.setItem(modeKey, mode);
    } catch {
      /* optional */
    }
    render({ scroll: true });
  });
  document.addEventListener("keydown", (event) => {
    if (
      event.defaultPrevented ||
      event.ctrlKey ||
      event.altKey ||
      event.metaKey ||
      event.shiftKey ||
      event.target.closest("input,select,textarea,button,a,[contenteditable]")
    )
      return;
    let index;
    if (event.key === "ArrowRight") index = current + (mode === "rtl" ? -1 : 1);
    else if (event.key === "ArrowLeft")
      index = current + (mode === "rtl" ? 1 : -1);
    else if (event.key === "Home") index = 0;
    else if (event.key === "End") index = figures.length - 1;
    else return;
    event.preventDefault();
    go(index);
  });
  // Track the page at the center of the reading viewport beneath sticky controls.
  // This matches the reader's document scroll-padding plus each figure's margin.
  function observePosition() {
    scrollFrame = 0;
    if (mode !== "vertical") return;
    const toolbarBottom = root
      .querySelector(".reader-toolbar")
      .getBoundingClientRect().bottom;
    const line =
      toolbarBottom + Math.max(0, window.innerHeight - toolbarBottom) / 2;
    let best = 0;
    for (let index = 0; index < figures.length; index++) {
      if (figures[index].getBoundingClientRect().top <= line) best = index;
      else break;
    }
    if (best !== current) {
      current = best;
      render({ scroll: false, announce: false });
    }
  }
  window.addEventListener(
    "scroll",
    () => {
      if (!scrollFrame) scrollFrame = requestAnimationFrame(observePosition);
    },
    { passive: true },
  );
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "hidden") {
      saveLocal(current + 1);
      clearTimeout(saveTimer);
      void flush();
    }
  });
  window.addEventListener("pagehide", () => {
    saveLocal(current + 1);
    clearTimeout(saveTimer);
    void flush();
  });
  // Preserve a portable local fallback even after restoring an account bookmark.
  saveLocal(current + 1);
}
