import { execFileSync } from "node:child_process";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";

// Include only modules compiled into the executable, not every unused driver in an upstream module graph.
const lines = execFileSync(
  "go",
  [
    "list",
    "-deps",
    "-f",
    "{{with .Module}}{{.Path}}|{{.Version}}|{{.Dir}}{{end}}",
    "./cmd/komanook",
  ],
  { encoding: "utf8" },
)
  .trim()
  .split("\n");
const modules = new Map();
for (const line of lines) {
  const [name, version, directory] = line.split("|");
  if (version && directory) modules.set(name, { version, directory });
}
await mkdir("licenses/go", { recursive: true });
const rows = [];
for (const [name, { version, directory }] of [...modules].sort(([a], [b]) =>
  a.localeCompare(b),
)) {
  let license;
  for (const candidate of ["LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING"]) {
    try {
      license = await readFile(path.join(directory, candidate));
      break;
    } catch {}
  }
  if (!license) throw new Error(`Missing license for compiled module ${name}`);
  const filename = name.replaceAll("/", "__") + ".txt";
  await writeFile(`licenses/go/${filename}`, license);
  rows.push(`| ${name} | ${version} | [Notice](go/${filename}) |`);
}
const goroot = execFileSync("go", ["env", "GOROOT"], {
  encoding: "utf8",
}).trim();
await writeFile(
  "licenses/go-standard-library.txt",
  await readFile(path.join(goroot, "LICENSE")),
);
await writeFile(
  "licenses/tailwindcss.txt",
  await readFile("node_modules/tailwindcss/LICENSE"),
);
await writeFile(
  "licenses/README.md",
  `# Third-party notices\n\nThe application code's licensing is separate from these upstream notices and the original [CC0 comic fixtures](../content/LICENSE.md). Browser source provenance is in [browser-assets.json](browser-assets.json). Regenerate compiled-module notices with \`npm run licenses\`.\n\n| Compiled Go module | Pinned version | License text |\n| --- | --- | --- |\n${rows.join("\n")}\n\nThe Go standard library has its [BSD notice](go-standard-library.txt). The compiled stylesheet uses [Tailwind CSS](tailwindcss.txt). Browser behavior uses [Datastar](datastar.txt) and [MannerHTML](mannerhtml.txt). The owned button was adapted from [shadcn-templ](shadcn-templ.txt). Development-only npm packages retain their license files in the lockfile-resolved packages; they are not shipped in the application container.\n`,
);
