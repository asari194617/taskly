import { readdir, readFile, stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const appRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const distRoot = path.join(appRoot, "dist");
const sourceRoot = path.join(appRoot, "src");

function fail(message) {
  console.error(`picomatch exposure check failed: ${message}`);
  process.exitCode = 1;
}

async function walkFiles(root) {
  const files = [];

  for (const entry of await readdir(root, { withFileTypes: true })) {
    const entryPath = path.join(root, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await walkFiles(entryPath)));
    } else if (entry.isFile()) {
      files.push(entryPath);
    }
  }

  return files;
}

const packageJson = JSON.parse(
  await readFile(path.join(appRoot, "package.json"), "utf8"),
);
const packageLock = JSON.parse(
  await readFile(path.join(appRoot, "package-lock.json"), "utf8"),
);
const astroConfig = await readFile(
  path.join(appRoot, "astro.config.mjs"),
  "utf8",
);

for (const dependencyType of [
  "dependencies",
  "devDependencies",
  "optionalDependencies",
  "peerDependencies",
]) {
  if (packageJson[dependencyType]?.picomatch) {
    fail(`picomatch must not be a direct ${dependencyType} entry`);
  }
}

const picomatchCopies = Object.entries(packageLock.packages ?? {})
  .filter(([packagePath]) => /(?:^|\/)node_modules\/picomatch$/.test(packagePath))
  .map(([packagePath, metadata]) => `${packagePath}@${metadata.version}`);

if (picomatchCopies.length === 0) {
  fail(
    "package-lock.json no longer contains picomatch; remove this temporary guard and reassess the risk acceptance",
  );
}

if (!/\boutput\s*:\s*["']static["']/.test(astroConfig)) {
  fail('Astro must keep an explicit output: "static" setting');
}

const importPattern =
  /(?:from\s+|import\s*(?:\(\s*)?|require\s*\(\s*)["']picomatch(?:[\/"'])/;
for (const sourceFile of await walkFiles(sourceRoot)) {
  const contents = await readFile(sourceFile, "utf8");
  if (importPattern.test(contents)) {
    fail(`${path.relative(appRoot, sourceFile)} imports picomatch directly`);
  }
}

let distAvailable = true;
try {
  if (!(await stat(distRoot)).isDirectory()) {
    fail("dist is not a directory; run the static build before this check");
    distAvailable = false;
  }
} catch {
  fail("dist is missing; run the static build before this check");
  distAvailable = false;
}

const bundledSignatures = [
  "analyzeRepeatedExtglob",
  "maxExtglobRecursion",
  "picomatch/lib/parse",
];
const textExtensions = new Set([
  ".cjs",
  ".css",
  ".html",
  ".js",
  ".json",
  ".map",
  ".mjs",
]);

for (const outputFile of distAvailable ? await walkFiles(distRoot) : []) {
  if (!textExtensions.has(path.extname(outputFile))) continue;
  const contents = await readFile(outputFile, "utf8");
  const signature = bundledSignatures.find((candidate) =>
    contents.includes(candidate),
  );
  if (signature) {
    fail(
      `${path.relative(appRoot, outputFile)} contains picomatch signature ${signature}`,
    );
  }
}

if (process.exitCode) process.exit(process.exitCode);

console.log(
  `Verified ${picomatchCopies.length} transitive picomatch copies are confined to the static build toolchain and absent from dist.`,
);
