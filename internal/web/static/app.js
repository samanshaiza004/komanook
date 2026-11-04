import { MannerTabs } from "./vendor/manner/tabs.js";
if (!customElements.get("manner-tabs"))
  customElements.define("manner-tabs", MannerTabs);
