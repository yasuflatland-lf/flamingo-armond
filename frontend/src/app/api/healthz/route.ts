import { NextResponse } from "next/server";
import { gqlFetch } from "@/lib/apollo/server";
import { HealthzQuery } from "./queries";

type HealthzResponse = { ok: true; backend: string } | { ok: false; error: string };

export async function GET() {
  try {
    const data = await gqlFetch(HealthzQuery, { revalidate: 0 });
    const body: HealthzResponse = { ok: true, backend: data.health };
    return NextResponse.json(body);
  } catch (err) {
    console.error("[healthz] backend health check failed:", err);
    const body: HealthzResponse = {
      ok: false,
      error: err instanceof Error ? err.message : String(err),
    };
    return NextResponse.json(body, { status: 503 });
  }
}
