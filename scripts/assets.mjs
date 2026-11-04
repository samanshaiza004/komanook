import { mkdir, readFile, writeFile, copyFile } from "node:fs/promises";
import sharp from "sharp";
import { createHash } from "node:crypto";

// All artwork and dialogue below is original, CC0. See content/LICENSE.md.
const stories = [
  {
    slug: "last-signal",
    title: "The Last Signal",
    creator: "Komanook Studio",
    genre: "Science fiction",
    format: "Manga",
    colors: ["#1e2546", "#717fc2", "#fdac83"],
    chapter: "Someone is still listening",
    synopsis:
      "Every night, a forgotten radio tower sends one note into the stars. Tonight, someone answers.",
    lines: [
      [
        "The tower had been quiet for a hundred years.",
        "Mira kept listening anyway.",
      ],
      ["At 02:17, the sky answered.", "Not words. A single, trembling note."],
      [
        "Mira turned the dial. “Are you still there?”",
        "The little light blinked twice.",
      ],
      [
        "Somewhere beyond the clouds, another room lit up.",
        "For the first time, neither of them was alone.",
      ],
    ],
  },
  {
    slug: "between-tides",
    title: "Between the Tides",
    creator: "Komanook Studio",
    genre: "Slice of life",
    format: "Webcomic",
    colors: ["#1d595c", "#7eaea7", "#fbd3a0"],
    chapter: "What the sea brings back",
    synopsis:
      "A seaside repair shop. A box of unsent letters. Some things find their way home with the tide.",
    lines: [
      [
        "Every morning, Nao swept the sea out of the shop.",
        "Every evening, it left something behind.",
      ],
      [
        "Today it was a tin full of letters.",
        "None had a stamp. All said: “To you.”",
      ],
      [
        "“I think these belong to the lighthouse.”",
        "Grandmother smiled. “I wondered when they would arrive.”",
      ],
      [
        "They read them while the kettle sang.",
        "Outside, the tide began its long journey home.",
      ],
    ],
  },
  {
    slug: "orbit-post",
    title: "Orbit Post",
    creator: "Komanook Studio",
    genre: "Adventure",
    format: "Manhwa",
    colors: ["#a94e46", "#e5a27e", "#f9e6b5"],
    chapter: "One very small delivery",
    synopsis:
      "The smallest courier in the solar system has a very important package. The address? A moving moon.",
    lines: [
      [
        "Package 44. One small seed.",
        "Destination: the moon that never stays still.",
      ],
      [
        "Pip checked the map. Then the sky.",
        "The moon was already late for tomorrow.",
      ],
      ["“Special delivery!”", "A tiny door opened in a very large crater."],
      [
        "The seed found a home in the moon dust.",
        "And the moon finally had a reason to stop.",
      ],
    ],
  },
];

const escape = (value) =>
  value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
function scene(story, variant, width = 1000, height = 1400) {
  const [ink, middle, warm] = story.colors;
  const stars = Array.from(
    { length: 36 },
    (_, i) =>
      `<circle cx="${(i * 137 + 57) % 1000}" cy="${(i * 79 + 31) % 650}" r="${(i % 3) + 1}" fill="${warm}" opacity=".7"/>`,
  ).join("");
  let subject = "";
  if (story.slug === "last-signal")
    subject = `<path d="M0 920L230 680 420 870 680 640 1000 900V1400H0Z" fill="${middle}"/><path d="M0 1100L350 800 580 1000 1000 760V1400H0Z" fill="${ink}"/><path d="M570 970L655 400 740 970M600 780h110M625 620h58M645 470h22" stroke="${warm}" stroke-width="13" fill="none"/><circle cx="655" cy="390" r="18" fill="${warm}"/><path d="M590 330Q655 265 720 330M552 280Q655 175 758 280" fill="none" stroke="${warm}" stroke-width="6" opacity=".6"/><path d="M300 1090v-155q0-70 40-70t40 70v155z" fill="${warm}"/><circle cx="340" cy="830" r="30" fill="${warm}"/><path d="M285 1100h110" stroke="${warm}" stroke-width="10"/>`;
  else if (story.slug === "between-tides")
    subject = `<circle cx="760" cy="400" r="120" fill="${warm}"/><path d="M0 740Q170 640 350 740T700 740T1050 740V1400H0Z" fill="${middle}"/><path d="M0 900Q170 800 350 900T700 900T1050 900M0 1040Q180 950 400 1040T1000 1040" fill="none" stroke="${warm}" stroke-width="5"/><path d="M100 1170L100 830 360 660 620 830V1170Z" fill="${ink}"/><path d="M70 840L360 630 650 840" stroke="${warm}" stroke-width="20" fill="none"/><path d="M200 900h110v130H200zm190 0h110v270H390z" fill="${warm}"/><path d="M790 780L820 510h70l30 270z" fill="${ink}"/><path d="M815 510v-80h80v80z" fill="${warm}"/>`;
  else
    subject = `<circle cx="700" cy="590" r="230" fill="${warm}"/><ellipse cx="700" cy="590" rx="330" ry="60" transform="rotate(-27 700 590)" stroke="${middle}" stroke-width="30" fill="none"/><circle cx="630" cy="510" r="35" fill="${middle}" opacity=".6"/><circle cx="765" cy="650" r="65" fill="${middle}" opacity=".6"/><path d="M200 1020l90-310 90 310z" fill="${warm}"/><circle cx="290" cy="810" r="37" fill="${ink}"/><path d="M230 1030l60 170 60-170" fill="${middle}"/><path d="M80 1200L190 1120M390 1250L430 1180" stroke="${warm}" stroke-width="6"/>`;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 1000 1400" preserveAspectRatio="xMidYMid slice"><rect width="1000" height="1400" fill="${ink}"/>${stars}<g transform="translate(${variant * 6} ${variant * 4})">${subject}</g></svg>`;
}
function wrappedText(text, y, options = {}) {
  const max = options.max || 35;
  const words = text.split(" ");
  const lines = [];
  let line = "";
  for (const word of words) {
    if ((line + " " + word).trim().length > max) {
      lines.push(line);
      line = word;
    } else line = (line + " " + word).trim();
  }
  if (line) lines.push(line);
  return `<text x="${options.x || 70}" y="${y}" font-family="sans-serif" font-size="${options.size || 32}" font-weight="${options.weight || 500}" fill="${options.fill || "#fff"}">${lines.map((l, i) => `<tspan x="${options.x || 70}" dy="${i === 0 ? 0 : (options.size || 32) * 1.3}">${escape(l)}</tspan>`).join("")}</text>`;
}

await mkdir("internal/web/static/comics", { recursive: true });
await mkdir("internal/web/static/vendor/manner", { recursive: true });
await mkdir("licenses", { recursive: true });
for (const file of ["tabs.js", "logic.js", "shared.js"]) {
  const text = await readFile(`node_modules/mannerhtml/dist/${file}`, "utf8");
  await writeFile(
    `internal/web/static/vendor/manner/${file}`,
    text.replace(/\/\/# sourceMappingURL=.*$/m, ""),
  );
}
await copyFile("node_modules/mannerhtml/LICENSE", "licenses/mannerhtml.txt");
const fixtures = [];
const renderArtwork = process.argv.includes("--art");
for (const story of stories) {
  const cover = scene(story, 0, 400, 600).replace(
    "</svg>",
    `<rect x="40" y="55" width="920" height="260" rx="12" fill="${story.colors[0]}" opacity=".9"/>${wrappedText(story.title.toUpperCase(), 140, { max: 14, size: 70, weight: 800 })}${wrappedText("AN ORIGINAL KOMANOOK COMIC", 1310, { size: 21, max: 40 })}</svg>`,
  );
  if (renderArtwork) {
    await sharp(Buffer.from(cover))
      .webp({ quality: 88 })
      .toFile(`internal/web/static/comics/${story.slug}-cover.webp`);
  }
  const pages = [];
  for (let i = 0; i < story.lines.length; i++) {
    const lines = story.lines[i];
    const svg = scene(story, i).replace(
      "</svg>",
      `<rect x="30" y="30" width="940" height="220" rx="8" fill="#fffdf5"/>${wrappedText(lines[0], 105, { fill: story.colors[0], max: 42, size: 32 })}<rect x="30" y="1160" width="940" height="210" rx="8" fill="#fffdf5"/>${wrappedText(lines[1], 1230, { fill: story.colors[0], max: 42, size: 32 })}<text x="920" y="1340" font-family="sans-serif" font-size="24" fill="${story.colors[0]}">${i + 1}</text></svg>`,
    );
    const path = `comics/${story.slug}-${i + 1}.webp`;
    if (renderArtwork) {
      await sharp(Buffer.from(svg))
        .webp({ quality: 90 })
        .toFile(`internal/web/static/${path}`);
    }
    pages.push({
      number: i + 1,
      image: path,
      width: 1000,
      height: 1400,
      transcript: `Page ${i + 1}. ${lines.join(" ")} ${story.slug === "last-signal" ? "An illustrated radio tower above indigo mountains, a small figure listening beneath the stars." : story.slug === "between-tides" ? "An illustrated seaside cottage and lighthouse beside rolling green waves." : "An illustrated tiny rocket approaching a ringed moon."}`,
    });
  }
  fixtures.push({
    ...story,
    cover: `comics/${story.slug}-cover.webp`,
    rights: "Original Komanook fixture · CC0 1.0",
    pages,
    lines: undefined,
    colors: undefined,
  });
}
await mkdir("content", { recursive: true });
await writeFile(
  "content/fixtures.json",
  JSON.stringify(fixtures, null, 2) + "\n",
);
const datastar = await readFile("internal/web/static/vendor/datastar-1.0.4.js");
await writeFile(
  "licenses/browser-assets.json",
  JSON.stringify(
    {
      datastar: {
        version: "1.0.4",
        source:
          "https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.4/bundles/datastar.js",
        sha256: createHash("sha256").update(datastar).digest("hex"),
      },
      mannerhtml: {
        version: "0.3.0",
        source: "npm lockfile integrity; tabs, logic and shared only",
      },
      shadcn: {
        commit: "b7074c1dc2cb669e916b3a51ddf8152eba356fa9",
        source: "components/button/button.templ",
        changes:
          "Native button branch only; reduced properties; Komanook CSS; no class merge runtime.",
      },
    },
    null,
    2,
  ) + "\n",
);
