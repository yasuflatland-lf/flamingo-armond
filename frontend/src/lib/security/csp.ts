type CspSource = string | null | undefined | false;

type CspDirectiveMap = Partial<Record<string, readonly CspSource[]>>;

export type HtmlCspOptions = {
  nonce: string;
  supabaseUrl: string;
  reportUri?: string;
  reportTo?: string;
  speedInsightsOrigin?: string;
  // Turbopack/React dev mode needs the 'unsafe-eval' source for HMR and React refresh; production never does.
  allowUnsafeEval?: boolean;
};

const DEFAULT_REPORT_URI = "/api/csp-report";
const DEFAULT_REPORT_TO = "csp-endpoint";
const DEFAULT_SPEED_INSIGHTS_ORIGIN = "https://vitals.vercel-insights.com";

const DIRECTIVE_ORDER = [
  "default-src",
  "base-uri",
  "object-src",
  "frame-ancestors",
  "form-action",
  "img-src",
  "style-src",
  "script-src",
  "connect-src",
  "report-uri",
  "report-to",
] as const;

function normalizeSources(sources: readonly CspSource[] | undefined): string[] {
  if (!sources) {
    return [];
  }

  const seen = new Set<string>();
  const normalized: string[] = [];

  for (const source of sources) {
    if (typeof source !== "string") {
      continue;
    }

    const trimmed = source.trim();
    if (trimmed.length === 0 || seen.has(trimmed)) {
      continue;
    }

    seen.add(trimmed);
    normalized.push(trimmed);
  }

  return normalized;
}

function serializeDirective(
  name: string,
  sources: readonly CspSource[] | undefined,
): string | null {
  const normalized = normalizeSources(sources);
  if (normalized.length === 0) {
    return null;
  }

  return `${name} ${normalized.join(" ")}`;
}

export function serializeCsp(directives: CspDirectiveMap): string {
  const orderedNames = [
    ...DIRECTIVE_ORDER.filter((name) => name in directives),
    ...Object.keys(directives)
      .filter((name) => !DIRECTIVE_ORDER.includes(name as (typeof DIRECTIVE_ORDER)[number]))
      .sort(),
  ];

  const serialized: string[] = [];
  for (const name of orderedNames) {
    const value = serializeDirective(name, directives[name]);
    if (value !== null) {
      serialized.push(value);
    }
  }

  return serialized.join("; ");
}

function parseUrlOrThrow(url: string): URL {
  try {
    return new URL(url);
  } catch {
    throw new Error(`[csp] invalid url: ${JSON.stringify(url)}`);
  }
}

function toOrigin(url: string): string {
  return parseUrlOrThrow(url).origin;
}

function toWebSocketOrigin(url: string): string {
  const parsed = parseUrlOrThrow(url);
  if (parsed.protocol === "https:") {
    parsed.protocol = "wss:";
  } else if (parsed.protocol === "http:") {
    parsed.protocol = "ws:";
  }
  return parsed.origin;
}

export function buildHtmlCsp({
  nonce,
  supabaseUrl,
  reportUri = DEFAULT_REPORT_URI,
  reportTo = DEFAULT_REPORT_TO,
  speedInsightsOrigin = DEFAULT_SPEED_INSIGHTS_ORIGIN,
  allowUnsafeEval = false,
}: HtmlCspOptions): string {
  if (nonce.trim().length === 0) {
    throw new Error("nonce is required");
  }

  const supabaseOrigin = toOrigin(supabaseUrl);
  const supabaseRealtimeOrigin = toWebSocketOrigin(supabaseUrl);

  return serializeCsp({
    "default-src": ["'self'"],
    "base-uri": ["'self'"],
    "object-src": ["'none'"],
    "frame-ancestors": ["'none'"],
    "form-action": ["'self'"],
    "img-src": ["'self'", "data:", "blob:", "https://lh3.googleusercontent.com"],
    "style-src": ["'self'", "'unsafe-inline'"],
    "script-src": ["'self'", `'nonce-${nonce}'`, allowUnsafeEval && "'unsafe-eval'"],
    "connect-src": ["'self'", supabaseOrigin, supabaseRealtimeOrigin, speedInsightsOrigin],
    "report-uri": [reportUri],
    "report-to": [reportTo],
  });
}
