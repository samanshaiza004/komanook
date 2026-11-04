export class BootstrapController {
    #observer = null;
    #timer = null;
    #controller = null;
    start(host, retry) {
        this.stop();
        const controller = new AbortController();
        this.#controller = controller;
        this.#observer = new MutationObserver(() => retry(false));
        this.#observer.observe(host, { childList: true, subtree: true });
        this.#timer = setTimeout(() => {
            this.#timer = null;
            retry(host.ownerDocument?.readyState !== "loading");
        }, 0);
        host.ownerDocument?.addEventListener("DOMContentLoaded", () => retry(true), {
            once: true,
            signal: controller.signal,
        });
    }
    stop() {
        this.#observer?.disconnect();
        this.#observer = null;
        if (this.#timer !== null)
            clearTimeout(this.#timer);
        this.#timer = null;
        this.#controller?.abort();
        this.#controller = null;
    }
}
export function ownedBy(host, element, baseClass) {
    let current = element;
    while (current) {
        if (current === host)
            return true;
        const registered = globalThis.customElements?.get(current.localName);
        if (current instanceof baseClass ||
            registered === baseClass ||
            (registered && registered.prototype instanceof baseClass))
            return false;
        current = current.parentElement;
    }
    return false;
}
export function discoverOwned(host, selector, baseClass) {
    return [...host.querySelectorAll(selector)].filter((element) => ownedBy(host, element, baseClass));
}
export function idTokens(value) {
    return (value || "").split(/\s+/).filter(Boolean);
}
export function addIdReferences(references, element, attributes) {
    if (element.id)
        references.add(element.id);
    for (const attribute of attributes) {
        for (const id of idTokens(element.getAttribute(attribute)))
            references.add(id);
    }
}
export function duplicateIdError(document, references) {
    if (!document)
        return null;
    const counts = new Map();
    for (const element of document.querySelectorAll("[id]")) {
        counts.set(element.id, (counts.get(element.id) || 0) + 1);
    }
    for (const id of references) {
        if ((counts.get(id) || 0) > 1) {
            return `Duplicate authored id "${id}" would make an accessibility relationship ambiguous.`;
        }
    }
    return null;
}
export function missingIdError(document, references) {
    if (!document)
        return null;
    for (const id of references) {
        if (!document.getElementById(id))
            return `An accessibility relationship references missing id "${id}".`;
    }
    return null;
}
export function describe(host, fallback) {
    return `<${host.localName || fallback}>`;
}
export function reportError(host, fallback, message) {
    console.error(`${describe(host, fallback)}: ${message}`, host);
}
