type NormalizedReportBody = {
  blockedUrl?: string;
  disposition?: string;
  documentUrl?: string;
  effectiveDirective?: string;
  lineNumber?: number;
  originalPolicy?: string;
  referrer?: string;
  sample?: string;
  statusCode?: number;
  violatedDirective?: string;
};

type NormalizedReport = {
  type: string;
  age?: number;
  url: string | null;
  userAgent: string | null;
  body: NormalizedReportBody;
};

type ReportEnvelope = {
  contentType: string;
  report: NormalizedReport;
};

function getContentTypeHeader(contentType: string | null) {
  return contentType?.split(";")[0]?.trim().toLowerCase() ?? "";
}

function getString(value: unknown) {
  return typeof value === "string" ? value : undefined;
}

function getNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function normalizeBody(source: Record<string, unknown>): NormalizedReportBody {
  const body: NormalizedReportBody = {};

  const blockedUrl = getString(source.blockedUrl ?? source["blocked-uri"] ?? source.blockedURL);
  if (blockedUrl !== undefined) body.blockedUrl = blockedUrl;

  const disposition = getString(source.disposition);
  if (disposition !== undefined) body.disposition = disposition;

  const documentUrl = getString(source.documentUrl ?? source["document-uri"] ?? source.documentURL);
  if (documentUrl !== undefined) body.documentUrl = documentUrl;

  const effectiveDirective = getString(
    source.effectiveDirective ?? source["effective-directive"],
  );
  if (effectiveDirective !== undefined) body.effectiveDirective = effectiveDirective;

  const lineNumber = getNumber(source.lineNumber ?? source["line-number"]);
  if (lineNumber !== undefined) body.lineNumber = lineNumber;

  const originalPolicy = getString(source.originalPolicy ?? source["original-policy"]);
  if (originalPolicy !== undefined) body.originalPolicy = originalPolicy;

  const referrer = getString(source.referrer);
  if (referrer !== undefined) body.referrer = referrer;

  const sample = getString(source.sample);
  if (sample !== undefined) body.sample = sample;

  const statusCode = getNumber(source.statusCode ?? source["status-code"]);
  if (statusCode !== undefined) body.statusCode = statusCode;

  const violatedDirective = getString(
    source.violatedDirective ?? source["violated-directive"],
  );
  if (violatedDirective !== undefined) body.violatedDirective = violatedDirective;

  return body;
}

function normalizeSingleReport(
  contentType: string,
  payload: Record<string, unknown>,
): ReportEnvelope | null {
  const cspReport = payload["csp-report"];
  const source = cspReport && typeof cspReport === "object" ? (cspReport as Record<string, unknown>) : payload;

  if (!source || typeof source !== "object") {
    return null;
  }

  const body = normalizeBody(source);
  const url = getString(source.url ?? source["document-uri"] ?? source.documentURL ?? body.documentUrl) ?? null;
  const userAgent = getString(source.userAgent ?? source.user_agent) ?? null;
  const type = getString(source.type) ?? "csp-violation";
  const age = getNumber(source.age);

  return {
    contentType,
    report: {
      type,
      age,
      url,
      userAgent,
      body,
    },
  };
}

function logReport(envelope: ReportEnvelope) {
  console.warn("[csp-report] received report", envelope);
}

function logInvalid(contentType: string, reason: string) {
  console.warn("[csp-report] invalid report payload", {
    contentType,
    reason,
  });
}

async function readBody(request: Request) {
  const raw = await request.text();
  if (!raw.trim()) {
    return { ok: false as const, reason: "empty_body" };
  }

  try {
    return { ok: true as const, value: JSON.parse(raw) as unknown };
  } catch {
    return { ok: false as const, reason: "invalid_json" };
  }
}

function normalizePayload(contentType: string, payload: unknown): ReportEnvelope[] | null {
  if (Array.isArray(payload)) {
    const entries = payload
      .map((entry) => {
        if (!entry || typeof entry !== "object" || Array.isArray(entry)) {
          return null;
        }

        const record = entry as Record<string, unknown>;
        const bodySource =
          record.body && typeof record.body === "object" && !Array.isArray(record.body)
            ? (record.body as Record<string, unknown>)
            : record;
        const normalized = normalizeSingleReport(contentType, {
          ...record,
          ...bodySource,
          body: undefined,
        });

        if (!normalized) {
          return null;
        }

        if (record.body && typeof record.body === "object" && !Array.isArray(record.body)) {
          normalized.report.body = normalizeBody(bodySource);
        }

        return normalized;
      })
      .filter((entry): entry is ReportEnvelope => entry !== null);

    return entries.length > 0 ? entries : null;
  }

  if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
    return null;
  }

  const normalized = normalizeSingleReport(contentType, payload as Record<string, unknown>);
  return normalized ? [normalized] : null;
}

export async function POST(request: Request) {
  const contentType = getContentTypeHeader(request.headers.get("content-type"));
  const body = await readBody(request);

  if (!body.ok) {
    logInvalid(contentType, body.reason);
    return new Response(null, { status: 204 });
  }

  const reports = normalizePayload(contentType, body.value);
  if (!reports) {
    logInvalid(contentType, "unexpected_shape");
    return new Response(null, { status: 204 });
  }

  for (const report of reports) {
    logReport(report);
  }

  return new Response(null, { status: 204 });
}
