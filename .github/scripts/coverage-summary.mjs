import { readFileSync, writeFileSync } from "node:fs";

const [kind, input, output] = process.argv.slice(2);
if (!kind || !input || !output) {
  throw new Error("Usage: coverage-summary.mjs <go|istanbul> <input> <output>");
}

let percent;
if (kind === "go") {
  const match = readFileSync(input, "utf8").match(/^total:\s+\(statements\)\s+([\d.]+)%\s*$/m);
  if (!match) throw new Error(`Go coverage total is missing from ${input}`);
  percent = Number(match[1]);
} else if (kind === "istanbul") {
  percent = JSON.parse(readFileSync(input, "utf8")).total?.lines?.pct;
} else {
  throw new Error(`Unknown coverage format: ${kind}`);
}

if (typeof percent !== "number" || !Number.isFinite(percent) || percent < 0 || percent > 100) {
  throw new Error(`Invalid coverage percentage: ${percent}`);
}
writeFileSync(output, JSON.stringify({ percent }) + "\n");
