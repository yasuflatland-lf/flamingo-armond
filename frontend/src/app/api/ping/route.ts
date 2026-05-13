import { NextResponse } from "next/server";

type PingResponse = { ok: true };

export async function GET() {
  const body: PingResponse = { ok: true };
  return NextResponse.json(body);
}
