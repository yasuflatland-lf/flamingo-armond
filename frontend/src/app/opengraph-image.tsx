import { ImageResponse } from "next/og";

// Generated Open Graph image (Next.js file convention). Pure code — the brand
// color matches viewport.themeColor in app/layout.tsx; no design asset needed.

export const size = { width: 1200, height: 630 };
export const contentType = "image/png";
export const alt = "Flamingo Armond — Remember more, study less.";

export default function OpengraphImage() {
  return new ImageResponse(
    <div
      style={{
        width: "100%",
        height: "100%",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: 24,
        backgroundColor: "#FF6F79",
        color: "#FFFFFF",
      }}
    >
      <div style={{ fontSize: 96, fontWeight: 700, letterSpacing: -2 }}>Flamingo Armond</div>
      <div style={{ fontSize: 40, opacity: 0.9 }}>Remember more, study less.</div>
    </div>,
    { ...size },
  );
}
