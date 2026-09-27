import { type MessageFormatElement, parse, TYPE } from "@formatjs/icu-messageformat-parser";
import { describe, expect, it } from "vitest";
import enMessages from "../../messages/en.json";
import jaMessages from "../../messages/ja.json";

type Locale = "en" | "ja";

function flatten(
  obj: unknown,
  prefix = "",
  out: Record<string, string> = {},
): Record<string, string> {
  for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
    const key = prefix ? `${prefix}.${k}` : k;
    if (v !== null && typeof v === "object") flatten(v, key, out);
    else out[key] = String(v);
  }
  return out;
}

/** Argument name -> the element types it appears as ("argument", "plural", "tag", ...). */
function icuArguments(message: string): Map<string, Set<string>> {
  const out = new Map<string, Set<string>>();
  const add = (name: string, kind: string) => out.set(name, (out.get(name) ?? new Set()).add(kind));
  const walk = (els: MessageFormatElement[]) => {
    for (const el of els) {
      if (el.type === TYPE.literal || el.type === TYPE.pound) continue;
      if (el.type === TYPE.tag) {
        add(`<${el.value}>`, "tag");
        walk(el.children);
        continue;
      }
      add(el.value, TYPE[el.type]);
      if (el.type === TYPE.plural || el.type === TYPE.select) {
        for (const opt of Object.values(el.options)) walk(opt.value);
      }
    }
  };
  walk(parse(message));
  return out;
}

const en = flatten(enMessages);
const ja = flatten(jaMessages);

describe("i18n catalog parity", () => {
  it("en.json and ja.json have the same flattened key set", () => {
    const enKeys = Object.keys(en).sort();
    const jaKeys = Object.keys(ja).sort();
    const onlyEn = enKeys.filter((k) => !(k in ja));
    const onlyJa = jaKeys.filter((k) => !(k in en));

    expect({ onlyEn, onlyJa }).toEqual({ onlyEn: [], onlyJa: [] });
    expect(enKeys).toEqual(jaKeys);
  });

  it("every key uses the same ICU arguments in both catalogs, except plural-only arguments ja may drop", () => {
    const violations: string[] = [];
    const argumentsOf = (key: string, locale: Locale, message: string) => {
      try {
        return icuArguments(message);
      } catch {
        violations.push(`${key}: unparsable (${locale})`);
        return undefined;
      }
    };

    for (const [key, enMessage] of Object.entries(en)) {
      const jaMessage = ja[key];
      if (jaMessage === undefined) continue;
      const enArgs = argumentsOf(key, "en", enMessage);
      const jaArgs = argumentsOf(key, "ja", jaMessage);
      if (!enArgs || !jaArgs) continue;

      for (const name of jaArgs.keys()) {
        if (!enArgs.has(name)) violations.push(`${key}: ja-only argument ${name}`);
      }
      for (const [name, kinds] of enArgs) {
        if (jaArgs.has(name)) continue;
        // Plural-only is a rule, not a per-key allow-list: ja has no plural forms, so any
        // key may drop a plural selector without a list entry to maintain.
        if (kinds.size === 1 && kinds.has("plural")) continue;
        violations.push(`${key}: ja drops non-plural argument ${name}`);
      }
    }

    expect(violations).toEqual([]);
  });
});
