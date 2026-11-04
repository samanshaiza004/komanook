var _a;
import { edgeIndex, nextIndex, previousIndex } from "./logic.js";
import { BootstrapController, addIdReferences, describe, discoverOwned, duplicateIdError, ownedBy, reportError, } from "./shared.js";
const HTMLElementBase = globalThis.HTMLElement ??
    class {
    };
function isElement(value) {
    return value instanceof Element;
}
// Candidates for the panel tab-stop heuristic. The question is structural — can
// a keyboard user already reach something inside this panel with Tab? — and it
// must stay structural, because #syncPanelTabStop also runs against panels that
// are currently hidden, where any computed-style check reports nothing at all.
const FOCUS_CANDIDATES = "a[href], area[href], button, input, select, textarea, summary, iframe, audio[controls], video[controls], [contenteditable], [tabindex]";
function authoredTabIndex(element) {
    const authored = element.getAttribute("tabindex");
    if (authored === null)
        return null;
    const parsed = Number.parseInt(authored, 10);
    // An unparseable tabindex is ignored, leaving the element's own behavior.
    return Number.isNaN(parsed) ? null : parsed;
}
function isEditable(element) {
    const authored = element.getAttribute("contenteditable");
    return authored !== null && authored.toLowerCase() !== "false";
}
// :disabled carries the full "actually disabled" state, including controls
// disabled by an ancestor <fieldset disabled>, which the attribute alone misses.
function isDisabled(element) {
    return element.matches(":disabled");
}
function isDetailsSummary(element) {
    const parent = element.parentElement;
    if (!parent || parent.localName !== "details")
        return false;
    // Only the first summary child of a details element is a focus target.
    return parent.querySelector(":scope > summary") === element;
}
function isFocusableByDefault(element) {
    switch (element.localName) {
        case "a":
        case "area":
            return element.hasAttribute("href");
        case "button":
        case "select":
        case "textarea":
            return !isDisabled(element);
        case "input":
            return !isDisabled(element) && (element.getAttribute("type") || "").toLowerCase() !== "hidden";
        case "summary":
            return isDetailsSummary(element);
        case "iframe":
            return true;
        case "audio":
        case "video":
            return element.hasAttribute("controls");
        default:
            return isEditable(element);
    }
}
function isSequentiallyFocusable(element) {
    const tabIndex = authoredTabIndex(element);
    // HTML excludes every negative tabindex from sequential focus navigation, not
    // only -1, and a negative value also opts a natively focusable element out.
    if (tabIndex !== null)
        return tabIndex >= 0;
    return isFocusableByDefault(element);
}
function containsFocusTarget(panel) {
    for (const candidate of panel.querySelectorAll(FOCUS_CANDIDATES)) {
        if (isSequentiallyFocusable(candidate))
            return true;
    }
    return false;
}
function randomToken() {
    const cryptoObject = globalThis.crypto;
    if (cryptoObject?.randomUUID)
        return cryptoObject.randomUUID().slice(0, 8);
    return Math.random().toString(36).slice(2, 10);
}
export class MannerTabs extends HTMLElementBase {
    #initialized = false;
    #bootstrap = new BootstrapController();
    #abortController = null;
    #tablist;
    #tabs = [];
    #panels = [];
    #selectedIndex = 0;
    #profile;
    #activation;
    #orientation;
    #hiddenMode = "hidden";
    #instanceToken = `manner-${randomToken()}`;
    #nextTabId = 0;
    #nextPanelId = 0;
    #tabIds = new WeakMap();
    #panelIds = new WeakMap();
    #libraryPanelTabStops = new WeakSet();
    connectedCallback() {
        if (this.#initialized) {
            this.#bindRuntime();
            return;
        }
        const result = this.#tryUpgrade();
        if (result.state !== "pending")
            return;
        this.#bootstrap.start(this, (finalize) => this.#tryUpgrade(finalize));
    }
    disconnectedCallback() {
        this.#bootstrap.stop();
        this.#abortController?.abort();
        this.#abortController = null;
    }
    get selectedIndex() {
        return this.#selectedIndex;
    }
    set selectedIndex(index) {
        this.select(index);
    }
    select(index) {
        if (!Number.isInteger(index) || index < 0 || index >= this.#tabs.length) {
            throw new RangeError(`${describe(this, "manner-tabs")}: selectedIndex is out of range`);
        }
        this.#select(index, "programmatic");
    }
    refresh() {
        const selectedTab = this.#tabs[this.#selectedIndex];
        const result = this.#buildModel(selectedTab, true);
        if (result.state !== "valid")
            return;
        this.#commit(result.model, false);
    }
    #tryUpgrade(finalize = false) {
        const result = this.#buildModel(undefined, finalize);
        if (result.state === "valid") {
            this.#commit(result.model, true);
        }
        else if (result.state === "invalid") {
            this.#bootstrap.stop();
        }
        return result;
    }
    #buildModel(previousTab, finalize = false) {
        const tablists = discoverOwned(this, "[data-tablist]", _a);
        const tabs = discoverOwned(this, "[data-tab]", _a);
        const panels = discoverOwned(this, "[data-panel]", _a);
        const invalid = (message) => {
            reportError(this, "manner-tabs", message);
            return { state: "invalid" };
        };
        // A parser can call connectedCallback before all descendants exist. Keep
        // that state distinct from a malformed, fully constructed instance and
        // defer the final decision until parsing has left the loading state.
        if (!finalize &&
            (this.ownerDocument?.readyState === "loading" ||
                tablists.length === 0 || tabs.length === 0 || panels.length === 0)) {
            return { state: "pending" };
        }
        if (tablists.length !== 1) {
            return invalid(`Expected exactly one owned [data-tablist]; found ${tablists.length}.`);
        }
        if (tabs.length === 0) {
            return invalid("Expected at least one owned [data-tab].");
        }
        if (panels.length === 0) {
            return invalid("Expected at least one owned [data-panel].");
        }
        const tablist = tablists[0];
        if ((!tablist.hasAttribute("aria-label") || !tablist.getAttribute("aria-label")) &&
            (!tablist.hasAttribute("aria-labelledby") || !tablist.getAttribute("aria-labelledby"))) {
            return invalid("The owned tablist requires aria-label or aria-labelledby.");
        }
        if (tablist.hasAttribute("role") && tablist.getAttribute("role") !== "tablist") {
            return invalid("The tablist has a conflicting authored role.");
        }
        const allAnchors = tabs.every((tab) => tab.localName === "a");
        const allButtons = tabs.every((tab) => tab.localName === "button");
        if (!allAnchors && !allButtons) {
            return invalid("Tabs must be all <a> or all <button>; mixed or other elements are invalid.");
        }
        const profile = allAnchors ? "progressive" : "application";
        if (profile === "application" && tabs.length !== panels.length) {
            return invalid(`Application tabs require equal tab/panel counts; found ${tabs.length} and ${panels.length}.`);
        }
        const activation = this.getAttribute("data-activation") || "auto";
        if (activation !== "auto" && activation !== "manual") {
            return invalid(`Unsupported data-activation value "${activation}".`);
        }
        const hiddenMode = this.getAttribute("data-hidden") || "hidden";
        if (hiddenMode !== "hidden" && hiddenMode !== "until-found") {
            return invalid(`Unsupported data-hidden value "${hiddenMode}".`);
        }
        const dataOrientation = this.getAttribute("data-orientation");
        const configuredOrientation = dataOrientation || tablist.getAttribute("aria-orientation") || "horizontal";
        if (configuredOrientation !== "horizontal" && configuredOrientation !== "vertical") {
            return invalid(`Unsupported data-orientation value "${configuredOrientation}".`);
        }
        const authoredOrientation = tablist.getAttribute("aria-orientation");
        if (authoredOrientation && authoredOrientation !== "horizontal" && authoredOrientation !== "vertical") {
            return invalid(`Unsupported authored aria-orientation value "${authoredOrientation}".`);
        }
        if (dataOrientation && authoredOrientation && authoredOrientation !== dataOrientation) {
            return invalid("data-orientation and authored aria-orientation disagree.");
        }
        const orientation = (authoredOrientation || configuredOrientation);
        for (const tab of tabs) {
            if (tab.hasAttribute("disabled") || tab.getAttribute("aria-disabled") === "true") {
                return invalid("Disabled tabs are not supported.");
            }
            if (tab.getAttribute("role") && tab.getAttribute("role") !== "tab") {
                return invalid("A tab has a conflicting authored role.");
            }
            const authoredSelected = tab.getAttribute("aria-selected");
            if (authoredSelected && authoredSelected !== "true" && authoredSelected !== "false") {
                return invalid("aria-selected must be true or false when authored.");
            }
        }
        for (const panel of panels) {
            if (panel.getAttribute("role") && panel.getAttribute("role") !== "tabpanel") {
                return invalid("A panel has a conflicting authored role.");
            }
            if (panel.hasAttribute("aria-labelledby") && !panel.getAttribute("aria-labelledby")) {
                return invalid("A panel has an empty authored aria-labelledby.");
            }
        }
        const relevantIds = new Set();
        for (const element of [...tabs, ...panels]) {
            addIdReferences(relevantIds, element, ["aria-controls", "aria-labelledby"]);
        }
        addIdReferences(relevantIds, tablist, ["aria-labelledby"]);
        const duplicateError = duplicateIdError(this.ownerDocument, relevantIds);
        if (duplicateError)
            return invalid(duplicateError);
        const tabIds = [];
        const panelIds = [];
        const panelSet = new Set(panels);
        for (let index = 0; index < tabs.length; index += 1) {
            const tab = tabs[index];
            let panel;
            if (profile === "progressive") {
                const href = tab.getAttribute("href");
                if (!href || !/^#[^#/?]+$/.test(href)) {
                    return invalid("Progressive tabs require fragment-only href values such as #panel-id.");
                }
                const candidate = this.ownerDocument?.getElementById(href.slice(1));
                if (!(candidate instanceof HTMLElement) || !panelSet.has(candidate)) {
                    if (!finalize)
                        return { state: "pending" };
                    return invalid(`Tab href="${href}" targets a panel outside this tabs instance.`);
                }
                panel = candidate;
            }
            else {
                panel = panels[index];
            }
            if (!panel)
                return invalid("A tab could not be paired with a panel.");
            const panelId = this.#stableId(panel, "panel", panel.id);
            const tabId = this.#stableId(tab, "tab", tab.id);
            if (tab.hasAttribute("aria-controls") && tab.getAttribute("aria-controls") !== panelId) {
                return invalid("An authored aria-controls does not match the associated panel.");
            }
            if (panel.hasAttribute("aria-labelledby") && panel.getAttribute("aria-labelledby") !== tabId) {
                return invalid("An authored aria-labelledby does not match the associated tab.");
            }
            tabIds[index] = tabId;
            panelIds[index] = panelId;
        }
        const mappedPanels = tabs.map((tab, index) => {
            if (profile === "application")
                return panels[index];
            const href = tab.getAttribute("href");
            return this.ownerDocument?.getElementById(href.slice(1));
        });
        if (new Set(mappedPanels).size !== panels.length || mappedPanels.some((panel) => !panelSet.has(panel))) {
            return invalid("Every panel must be mapped to exactly one tab.");
        }
        const explicit = tabs
            .map((tab, index) => (tab.hasAttribute("data-selected") || tab.getAttribute("aria-selected") === "true" ? index : -1))
            .filter((index) => index >= 0);
        if (explicit.length > 1) {
            return invalid("Only one tab may be explicitly selected.");
        }
        let selectedIndex = explicit[0] ?? -1;
        if (selectedIndex < 0 && previousTab)
            selectedIndex = tabs.indexOf(previousTab);
        if (selectedIndex < 0 && profile === "progressive" && this.ownerDocument?.location.hash) {
            const hash = this.ownerDocument.location.hash;
            selectedIndex = tabs.findIndex((tab) => tab.getAttribute("href") === hash);
        }
        if (selectedIndex < 0)
            selectedIndex = 0;
        return { state: "valid", model: { tablist, tabs, panels: mappedPanels, profile, activation: activation, orientation, selectedIndex, tabIds, panelIds } };
    }
    #stableId(element, kind, authoredId) {
        const cache = kind === "tab" ? this.#tabIds : this.#panelIds;
        const existing = cache.get(element);
        if (existing)
            return existing;
        if (authoredId) {
            cache.set(element, authoredId);
            return authoredId;
        }
        let candidate = `${this.#instanceToken}-${kind}-${kind === "tab" ? this.#nextTabId++ : this.#nextPanelId++}`;
        let suffix = 0;
        while (this.ownerDocument?.getElementById(candidate)) {
            candidate = `${this.#instanceToken}-${kind}-${kind === "tab" ? this.#nextTabId++ : this.#nextPanelId++}-${suffix++}`;
        }
        cache.set(element, candidate);
        return candidate;
    }
    #commit(model, initial) {
        this.#bootstrap.stop();
        this.#tablist = model.tablist;
        this.#tabs = model.tabs;
        this.#panels = model.panels;
        this.#profile = model.profile;
        this.#activation = model.activation;
        this.#orientation = model.orientation;
        this.#hiddenMode = this.getAttribute("data-hidden") === "until-found" ? "until-found" : "hidden";
        this.#tabs.forEach((tab, index) => tab.id = tab.id || model.tabIds[index]);
        this.#panels.forEach((panel, index) => panel.id = panel.id || model.panelIds[index]);
        this.#tablist.setAttribute("role", "tablist");
        if (this.#orientation === "vertical")
            this.#tablist.setAttribute("aria-orientation", "vertical");
        this.#tabs.forEach((tab, index) => {
            tab.setAttribute("role", "tab");
            tab.setAttribute("aria-controls", this.#panels[index].id);
            tab.setAttribute("aria-selected", String(index === model.selectedIndex));
            tab.tabIndex = index === model.selectedIndex ? 0 : -1;
        });
        this.#panels.forEach((panel, index) => {
            panel.setAttribute("role", "tabpanel");
            panel.setAttribute("aria-labelledby", this.#tabs[index].id);
            this.#syncPanelTabStop(panel);
            this.#setPanelHidden(panel, index !== model.selectedIndex);
        });
        this.#selectedIndex = model.selectedIndex;
        this.#initialized = true;
        this.#bindRuntime();
        void initial;
    }
    #bindRuntime() {
        if (this.#abortController)
            return;
        const controller = new AbortController();
        this.#abortController = controller;
        this.addEventListener("click", this.#onClick, { signal: controller.signal });
        this.addEventListener("keydown", this.#onKeyDown, { signal: controller.signal });
        this.addEventListener("beforematch", this.#onBeforeMatch, { signal: controller.signal });
    }
    #tabFromTarget(target) {
        if (!isElement(target))
            return null;
        const tab = target.closest("[data-tab]");
        return tab && this.#tabs.includes(tab) && ownedBy(this, tab, _a) ? tab : null;
    }
    #onClick = (event) => {
        const tab = this.#tabFromTarget(event.target);
        if (!tab || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey)
            return;
        if (this.#profile === "progressive")
            event.preventDefault();
        this.#select(this.#tabs.indexOf(tab), "pointer");
    };
    #onKeyDown = (event) => {
        const tab = this.#tabFromTarget(event.target);
        if (!tab)
            return;
        const index = this.#tabs.indexOf(tab);
        let next = -1;
        if (event.key === "Home")
            next = edgeIndex("first", this.#tabs.length);
        else if (event.key === "End")
            next = edgeIndex("last", this.#tabs.length);
        else if (this.#orientation === "horizontal" && event.key === "ArrowRight")
            next = nextIndex(index, this.#tabs.length);
        else if (this.#orientation === "horizontal" && event.key === "ArrowLeft")
            next = previousIndex(index, this.#tabs.length);
        else if (this.#orientation === "vertical" && event.key === "ArrowDown")
            next = nextIndex(index, this.#tabs.length);
        else if (this.#orientation === "vertical" && event.key === "ArrowUp")
            next = previousIndex(index, this.#tabs.length);
        else if (this.#activation === "manual" && (event.key === "Enter" || event.key === " " || event.key === "Spacebar")) {
            event.preventDefault();
            this.#select(index, "keyboard");
            return;
        }
        if (next < 0)
            return;
        event.preventDefault();
        this.#tabs[next].focus();
        if (this.#activation === "auto")
            this.#select(next, "keyboard");
    };
    #onBeforeMatch = (event) => {
        const panel = event.target instanceof HTMLElement && this.#panels.includes(event.target) ? event.target : null;
        if (panel)
            this.#select(this.#panels.indexOf(panel), "programmatic");
    };
    #select(index, source) {
        if (index < 0 || index >= this.#tabs.length || index === this.#selectedIndex)
            return;
        this.#selectedIndex = index;
        this.#tabs.forEach((tab, current) => {
            tab.setAttribute("aria-selected", String(current === index));
            tab.tabIndex = current === index ? 0 : -1;
        });
        this.#panels.forEach((panel, current) => {
            this.#syncPanelTabStop(panel);
            this.#setPanelHidden(panel, current !== index);
        });
        this.dispatchEvent(new CustomEvent("manner-tabs-change", {
            bubbles: true,
            detail: { index, tab: this.#tabs[index], panel: this.#panels[index], source },
        }));
    }
    #syncPanelTabStop(panel) {
        const focusable = containsFocusTarget(panel);
        if (this.#libraryPanelTabStops.has(panel)) {
            if (panel.getAttribute("tabindex") !== "0") {
                this.#libraryPanelTabStops.delete(panel);
                return;
            }
            if (focusable) {
                this.#libraryPanelTabStops.delete(panel);
                panel.removeAttribute("tabindex");
            }
            return;
        }
        if (panel.hasAttribute("tabindex"))
            return;
        if (!focusable) {
            panel.tabIndex = 0;
            this.#libraryPanelTabStops.add(panel);
        }
    }
    #setPanelHidden(panel, hidden) {
        if (hidden)
            panel.setAttribute("hidden", this.#hiddenMode === "until-found" ? "until-found" : "");
        else
            panel.removeAttribute("hidden");
    }
}
_a = MannerTabs;
