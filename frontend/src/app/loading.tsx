import { BrandSplash } from "@/components/pwa/brand-splash";

/**
 * Root-level streaming/navigation fallback.
 *
 * The root redirect (`app/page.tsx`) blocks on a GraphQL fetch to the backend
 * before it can choose a destination. When the backend is cold-starting or
 * stopped that wait can last tens of seconds, and there is no page UI to stream
 * during it. This Suspense fallback is rendered entirely by the frontend host
 * with no backend dependency, so the branded spinner appears immediately — the
 * PWA launch experience the OS/manifest splash hands off to — instead of a blank
 * or black screen.
 *
 * It also backs any descendant route that lacks its own `loading.tsx`; routes
 * that ship a skeleton (cardgroups, learn, admin, cards/new) keep theirs, since
 * the nearest boundary wins.
 */
export default function Loading() {
  return <BrandSplash label="Loading" />;
}
