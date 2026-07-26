import { NextResponse } from "next/server";

const apiTarget = (process.env.API_PROXY_TARGET || "http://127.0.0.1:8080").replace(/\/$/, "");

export async function GET() {
  try {
    const response = await fetch(`${apiTarget}/health/ready`, { cache: "no-store" });
    if (!response.ok) {
      return NextResponse.json({ status: "unavailable" }, { status: 503 });
    }
    return NextResponse.json({ status: "ok" });
  } catch {
    return NextResponse.json({ status: "unavailable" }, { status: 503 });
  }
}
