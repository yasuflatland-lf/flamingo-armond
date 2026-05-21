# React JSX interpolation is not an XSS vector

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

React JSX `{expression}` syntax passes string values through `React.createElement` as text children. The reconciler inserts them into the DOM as text nodes via `Node.textContent`, not via `innerHTML`. The browser never parses these values as HTML.

## Why JSX `{var}` cannot execute injected HTML

Given a URL like `?error=%3Cimg+src%3Dx+onerror%3Dalert(1)%3E`, extracting the `error` param and rendering it via:

```tsx
const error = searchParams.get("error"); // "<img src=x onerror=alert(1)>"
return <p>{error}</p>;
```

produces the literal string `<img src=x onerror=alert(1)>` as visible text on the page. The angle brackets and attribute characters are displayed as characters; no element is created, no handler runs. This is the same behaviour regardless of how malicious the string appears.

Primary source: [React — JavaScript in JSX with Curly Braces](https://react.dev/learn/javascript-in-jsx-with-curly-braces). The React docs state that JSX children are text content, not HTML markup.

## The only React XSS vector is `dangerouslySetInnerHTML`

The prop `dangerouslySetInnerHTML` (value: `{ __html: string }`) instructs React to set `innerHTML` directly, bypassing the text-node path. Any content reaching this prop is parsed as HTML and arbitrary scripts can execute. The prop name is deliberately alarming.

If a value must flow into this prop, sanitize it with a library such as DOMPurify before use, and model the sanitized value with a branded type or a load-bearing JSDoc contract so the sanitization step is visible at every call site. See [`jsdoc-as-the-enforcer-of-pre-sanitized-invariants.md`](jsdoc-as-the-enforcer-of-pre-sanitized-invariants.md).

## Adjacent vectors that ARE concerns

JSX interpolation only governs the text-child path. Two adjacent patterns require different treatment:

- **URL context** — `<a href={value}>` where `value` is user-supplied. A `javascript:` URI executes on click. Validate or allowlist the scheme before interpolating into `href`, `src`, or `action`.
- **CSS context** — `style={{ backgroundImage: \`url(${value})\` }}` where `value` is user-supplied. CSS injection can load attacker-controlled resources. Validate or reject values before composing into CSS strings.

Neither of these involves JSX string interpolation in the sense of `React.createElement` text children; they are separate vectors with separate mitigations.

## Scope of this rule

This rule applies to **React JSX** components. Other templating or rendering layers in the same project have different escaping defaults:

- Server-rendered raw HTML (e.g. a Node route that writes `res.write(value)` directly) has no automatic escaping.
- MDX with `rawHTML` enabled parses HTML in Markdown source — treat it like the raw HTML injection path.
- Next.js `generateStaticParams` and server-action return values flow through React serialization, which is the same text-node path for string children.

**How to apply:** when a security review flags a JSX interpolation site as a potential XSS, confirm the value reaches the raw-HTML prop, an `href`/`src`/`action` attribute, or a CSS string before escalating. A plain `{value}` child in JSX is not a finding.
