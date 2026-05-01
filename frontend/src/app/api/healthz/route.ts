import { NextResponse } from "next/server";
import { graphql } from "@/generated";
import { gqlFetch } from "@/lib/apollo/server";

const HealthQuery = graphql(`
  query Healthz {
    health
  }
`);

export async function GET() {
  try {
    const data = await gqlFetch(HealthQuery, { revalidate: 0 });
    return NextResponse.json({ ok: true, backend: data.health });
  } catch (err) {
    return NextResponse.json(
      { ok: false, error: err instanceof Error ? err.message : String(err) },
      { status: 503 },
    );
  }
}
