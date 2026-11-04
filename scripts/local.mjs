import EmbeddedPostgres from "embedded-postgres";
import { spawn } from "node:child_process";
import { mkdir, mkdtemp, access, rm, open } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { once } from "node:events";

const mode = process.argv[2] || "dev";
if (!["dev", "test"].includes(mode)) throw new Error("Use dev or test");
const test = mode === "test";
await mkdir(".data", { recursive: true });
await mkdir("bin", { recursive: true });
const scratch = await mkdtemp(path.join(tmpdir(), "komanook-"));
const directory = test
  ? path.join(scratch, "postgres")
  : path.resolve(".data/postgres");
const port = test ? 54330 : 54329;
const pg = new EmbeddedPostgres({
  databaseDir: directory,
  port,
  user: "komanook",
  password: "local-komanook-only",
  authMethod: "scram-sha-256",
  persistent: !test,
  autoRehydrateSymlinks: true,
  postgresFlags: ["-h", "127.0.0.1", "-k", scratch],
  onLog: () => {},
  onError: (message) => {
    if (/FATAL|ERROR/.test(String(message))) console.error(String(message));
  },
});
const dsn = `postgres://komanook:local-komanook-only@127.0.0.1:${port}/komanook?sslmode=disable`;
const env = {
  ...process.env,
  APP_ENV: test ? "test" : "development",
  HTTP_ADDR: "127.0.0.1:8080",
  BASE_URL: "http://localhost:8080",
  DATABASE_URL: dsn,
  TEST_DATABASE_URL: dsn,
  MEDIA_DIR: test ? path.join(scratch, "media") : path.resolve(".data/media"),
};
let server;
let stopping = false;
async function stop() {
  if (stopping) return;
  stopping = true;
  if (server && server.exitCode === null) {
    server.kill("SIGTERM");
    await Promise.race([
      once(server, "exit"),
      new Promise((resolve) => setTimeout(resolve, 16000).unref()),
    ]);
    if (server.exitCode === null) server.kill("SIGKILL");
  }
  await pg.stop();
  await rm(scratch, { recursive: true, force: true });
}
process.once("SIGINT", () => {
  void stop().finally(() => process.exit(0));
});
process.once("SIGTERM", () => {
  void stop().finally(() => process.exit(0));
});
function run(command, args, options = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      env,
      stdio: options.input ? ["pipe", "inherit", "inherit"] : "inherit",
    });
    if (options.input) child.stdin.end(options.input);
    child.once("error", reject);
    child.once("exit", (code) =>
      code === 0 ? resolve() : reject(new Error(`${command} exited ${code}`)),
    );
  });
}
try {
  try {
    await access(path.join(directory, "PG_VERSION"));
  } catch {
    await pg.initialise();
  }
  await pg.start();
  try {
    await pg.createDatabase("komanook");
  } catch (error) {
    if (error.code !== "42P04") throw error;
  }
  await run("go", [
    "build",
    "-trimpath",
    "-o",
    "bin/komanook",
    "./cmd/komanook",
  ]);
  await run("./bin/komanook", ["migrate"]);
  await run("./bin/komanook", ["seed"]);
  await run(
    "./bin/komanook",
    ["user", "--email", "reader@komanook.test", "--role", "reader"],
    { input: "a quiet place to read\n" },
  );
  await run(
    "./bin/komanook",
    ["user", "--email", "operator@komanook.test", "--role", "admin"],
    { input: "a quiet place to work\n" },
  );
  if (test) await run("go", ["test", "-race", "./..."]);
  const log = await open(".data/server.log", "w");
  server = spawn("./bin/komanook", ["serve"], {
    env,
    stdio: ["ignore", log.fd, log.fd],
  });
  await log.close();
  server.once("error", (error) => console.error(error));
  let ready = false;
  for (let attempt = 0; attempt < 100; attempt++) {
    if (server.exitCode !== null)
      throw new Error("Server stopped; see .data/server.log");
    try {
      const response = await fetch("http://localhost:8080/health/ready");
      if (response.ok) {
        ready = true;
        break;
      }
    } catch {}
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  if (!ready) throw new Error("Server readiness timed out");
  console.log(
    "Komanook is ready at http://localhost:8080 — local fixture accounts only.",
  );
  if (test) {
    await run("npm", ["run", "test:browser"]);
    await run("npm", ["run", "bench"]);
  } else await once(server, "exit");
} catch (error) {
  console.error(error);
  process.exitCode = 1;
} finally {
  await stop();
}
