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

/** Argument name -> the element types it appears as ("argument", "plural", "pound", "tag", ...). */
function icuArguments(message: string): Map<string, Set<string>> {
  const out = new Map<string, Set<string>>();
  const add = (name: string, kind: string) => out.set(name, (out.get(name) ?? new Set()).add(kind));
  const walk = (els: MessageFormatElement[], enclosingPlural?: string) => {
    for (const el of els) {
      if (el.type === TYPE.literal) continue;
      if (el.type === TYPE.pound) {
        if (enclosingPlural) add(enclosingPlural, "pound");
        continue;
      }
      if (el.type === TYPE.tag) {
        add(`<${el.value}>`, "tag");
        walk(el.children, enclosingPlural);
        continue;
      }
      add(el.value, TYPE[el.type]);
      if (el.type === TYPE.plural || el.type === TYPE.select) {
        for (const opt of Object.values(el.options)) {
          walk(opt.value, el.type === TYPE.plural ? el.value : enclosingPlural);
        }
      }
    }
  };
  walk(parse(message));
  return out;
}

const en = flatten(enMessages);
const ja = flatten(jaMessages);

function argumentViolations(en: Record<string, string>, ja: Record<string, string>): string[] {
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
      // Exempting a plural that renders # would let ja drop a displayed count.
      if (kinds.size === 1 && kinds.has("plural")) continue;
      violations.push(`${key}: ja drops non-plural argument ${name}`);
    }
  }

  return violations;
}

describe("i18n catalog parity", () => {
  it("en.json and ja.json have the same flattened key set", () => {
    const onlyEn = Object.keys(en).filter((k) => !Object.hasOwn(ja, k));
    const onlyJa = Object.keys(ja).filter((k) => !Object.hasOwn(en, k));

    expect({ onlyEn, onlyJa }).toEqual({ onlyEn: [], onlyJa: [] });
  });

  it("every key uses the same ICU arguments in both catalogs, except plural-only arguments ja may drop", () => {
    expect(argumentViolations(en, ja)).toEqual([]);
  });

  it("allows selector-only plurals but rejects dropped rendered counts, including in tags", () => {
    expect(
      argumentViolations({ k: "{count, plural, one {card} other {cards}}" }, { k: "cards" }),
    ).toEqual([]);
    expect(
      argumentViolations({ k: "{count, plural, one {# card} other {# cards}}" }, { k: "cards" }),
    ).toEqual(["k: ja drops non-plural argument count"]);
    expect(
      argumentViolations({ k: "{count, plural, other {<b># items</b>}}" }, { k: "<b>items</b>" }),
    ).toEqual(["k: ja drops non-plural argument count"]);
  });

  it("reports ja-only arguments, dropped or renamed tags, select-branch arguments and unparsable messages", () => {
    expect(argumentViolations({ k: "Hi" }, { k: "Hi {name}" })).toEqual([
      "k: ja-only argument name",
    ]);
    expect(argumentViolations({ k: "<b>x</b>" }, { k: "x" })).toEqual([
      "k: ja drops non-plural argument <b>",
    ]);
    expect(argumentViolations({ k: "<b>x</b>" }, { k: "<i>x</i>" })).toEqual([
      "k: ja-only argument <i>",
      "k: ja drops non-plural argument <b>",
    ]);
    expect(
      argumentViolations(
        { k: "{g, select, a {{name} A} other {B}}" },
        { k: "{g, select, other {B}}" },
      ),
    ).toEqual(["k: ja drops non-plural argument name"]);
    expect(argumentViolations({ k: "Hi {name" }, { k: "Hi {name}" })).toEqual([
      "k: unparsable (en)",
    ]);
    expect(argumentViolations({ k: "Hi {name}" }, { k: "Hi {name" })).toEqual([
      "k: unparsable (ja)",
    ]);
  });
});
