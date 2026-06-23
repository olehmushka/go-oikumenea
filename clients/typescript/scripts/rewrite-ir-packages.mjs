#!/usr/bin/env node
// conjure-typescript rejects Conjure type packages with fewer than 3 dot-segments
// ("Package should have at least 3 segments"). The go-oikumenea contract uses 2-segment
// packages (`oikumenea.<module>`), which the Go generator (godel-conjure-plugin) accepts but
// conjure-typescript does not. Package names are a code-organization concern only — they do NOT
// appear on the wire (endpoint paths, service names and JSON shapes are unchanged) — so we rewrite
// `oikumenea.<module>` -> `oikumenea.api.<module>` in the IR before generating the TS SDK. This is a
// pure, deterministic transform of a derived artifact (the IR), never of the source contract.
//
// Usage: node rewrite-ir-packages.mjs <in-ir.json> <out-ir.json>
import { readFileSync, writeFileSync } from "node:fs";

const [, , inPath, outPath] = process.argv;
if (!inPath || !outPath) {
  console.error("usage: rewrite-ir-packages.mjs <in-ir.json> <out-ir.json>");
  process.exit(2);
}

const PREFIX = "oikumenea.";
const NEW_PREFIX = "oikumenea.api.";
let count = 0;

function walk(node) {
  if (Array.isArray(node)) {
    node.forEach(walk);
    return;
  }
  if (node && typeof node === "object") {
    for (const key of Object.keys(node)) {
      const val = node[key];
      if (
        key === "package" &&
        typeof val === "string" &&
        val.startsWith(PREFIX) &&
        !val.startsWith(NEW_PREFIX)
      ) {
        node[key] = NEW_PREFIX + val.slice(PREFIX.length);
        count++;
      } else {
        walk(val);
      }
    }
  }
}

const ir = JSON.parse(readFileSync(inPath, "utf8"));
walk(ir);
writeFileSync(outPath, JSON.stringify(ir));
console.error(`rewrite-ir-packages: rewrote ${count} package fields -> ${outPath}`);
