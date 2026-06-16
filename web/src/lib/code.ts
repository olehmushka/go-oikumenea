// Client-side `code` auto-generation for the create forms (person, position). When the operator
// leaves the code field empty we derive a stable, locale-agnostic code from the entity's name:
// transliterate Cyrillic -> Latin, lowercase, kebab-case, then append "-" + a short nanoid suffix
// for uniqueness. The result satisfies the D-Code shape (no whitespace, <=128 chars).

import { customAlphabet } from "nanoid";

// Lowercase base36 suffix — no separators, so the only "-" in a code separates the slug from the
// suffix. Length 6 ≈ 2.2B combinations, ample to avoid practical collisions per name.
const nano = customAlphabet("0123456789abcdefghijklmnopqrstuvwxyz", 6);

// Ukrainian (KMU-2010 official romanization) plus the extra Russian letters. Keys are lowercase;
// callers lowercase before lookup. Unmapped runes pass through and are handled by slugify's filter.
const CYRILLIC: Record<string, string> = {
  а: "a", б: "b", в: "v", г: "h", ґ: "g", д: "d", е: "e", є: "ie", ж: "zh",
  з: "z", и: "y", і: "i", ї: "i", й: "i", к: "k", л: "l", м: "m", н: "n",
  о: "o", п: "p", р: "r", с: "s", т: "t", у: "u", ф: "f", х: "kh", ц: "ts",
  ч: "ch", ш: "sh", щ: "shch", ь: "", ю: "iu", я: "ia",
  // Russian-specific
  ё: "e", ы: "y", э: "e", ъ: "",
};

/** Transliterate Cyrillic to Latin and lowercase; unmapped characters pass through unchanged. */
export function transliterate(input: string): string {
  let out = "";
  for (const ch of input.toLowerCase()) out += CYRILLIC[ch] ?? ch;
  return out;
}

/** Lowercase, transliterated, kebab-cased slug (a-z0-9 and single dashes, trimmed). */
export function slugify(input: string): string {
  return transliterate(input)
    .normalize("NFKD")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

/** A full auto-code: `${slug}-${suffix}`, or just the suffix when the name has no usable characters. */
export function generateCode(name: string): string {
  const slug = slugify(name);
  return slug ? `${slug}-${nano()}` : nano();
}

/** A stable per-form suffix; pair with slugify(name) for a live-updating preview that doesn't churn. */
export function newSuffix(): string {
  return nano();
}
