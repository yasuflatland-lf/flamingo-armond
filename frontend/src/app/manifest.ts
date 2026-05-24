import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "flamingo-armond",
    short_name: "flamingo",
    description: "Swiping flashcard app.",
    start_url: "/",
    display: "standalone",
    background_color: "#FF6F79",
    theme_color: "#FF6F79",
    icons: [
      { src: "/icon-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
      { src: "/icon-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
      { src: "/icon-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  };
}
