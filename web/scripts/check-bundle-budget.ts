import { gzipSync } from "node:zlib";
import { readFileSync, readdirSync } from "node:fs";
import { basename, join } from "node:path";

type ManifestEntry = {
  file: string;
  imports?: string[];
};

const outputDirectory = ".svelte-kit/output/client";
const manifest = JSON.parse(
  readFileSync(join(outputDirectory, ".vite/manifest.json"), "utf8"),
) as Record<string, ManifestEntry>;

const nodeDirectory = ".svelte-kit/generated/client-optimized/nodes";
const nodes = Object.fromEntries(
  readdirSync(nodeDirectory)
    .filter((file) => file.endsWith(".js"))
    .map((file) => {
      const source = readFileSync(join(nodeDirectory, file), "utf8");
      const route = source.match(/src\/routes\/(.+\.svelte)/)?.[1];
      return route ? [route, Number.parseInt(basename(file, ".js"), 10)] : [];
    })
    .filter((entry) => entry.length === 2),
) as Record<string, number>;

const manifestNode = (node: number) =>
  Object.keys(manifest).find((key) => key.endsWith(`/nodes/${node}.js`))!;

function dependencies(entries: string[]) {
  const result = new Set<string>();
  const pending = [...entries];
  while (pending.length) {
    const entry = pending.pop()!;
    if (result.has(entry)) continue;
    result.add(entry);
    pending.push(...(manifest[entry].imports ?? []));
  }
  return result;
}

function gzipKiB(entries: Set<string>) {
  const files = new Set([...entries].map((entry) => manifest[entry].file));
  return (
    [...files].reduce(
      (total, file) =>
        total +
        gzipSync(readFileSync(join(outputDirectory, file)), { level: 9 })
          .length,
      0,
    ) / 1024
  );
}

const rootLayout = manifestNode(nodes["+layout.svelte"]);
const appLayout = manifestNode(nodes["(app)/+layout.svelte"]);
const routes = Object.keys(nodes).filter((route) =>
  route.endsWith("+page.svelte"),
);

let failed = false;
for (const route of routes) {
  const layouts = route.startsWith("(app)/")
    ? [rootLayout, appLayout]
    : [rootLayout];
  const limit =
    route === "(app)/services/[folder]/+page.svelte"
      ? 180
      : route === "login/+page.svelte"
        ? 45
        : 150;
  const size = gzipKiB(dependencies([...layouts, manifestNode(nodes[route])]));
  console.log(`${route}: ${size.toFixed(1)} KiB / ${limit} KiB gzip`);
  if (size > limit) failed = true;
}

const initialEntries = dependencies([
  rootLayout,
  appLayout,
  ...routes.map((route) => manifestNode(nodes[route])),
]);
for (const pattern of [
  "src/lib/components/app/code-editor.svelte",
  "@wterm+dom",
  "@wterm+ghostty",
  "src/lib/i18n/messages/",
]) {
  if ([...initialEntries].some((entry) => entry.includes(pattern))) {
    console.error(`${pattern} must remain lazy-loaded`);
    failed = true;
  }
}

if (failed) {
  process.exitCode = 1;
}
