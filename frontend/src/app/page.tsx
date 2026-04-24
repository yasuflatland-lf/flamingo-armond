import { graphql } from "@/generated";
import { gqlFetch } from "@/lib/apollo/server";

const HealthQuery = graphql(`
  query Health {
    health
  }
`);

export default async function HomePage() {
  const data = await gqlFetch(HealthQuery, { revalidate: 0 });
  return (
    <main className="flex min-h-screen items-center justify-center p-8">
      <h1 className="text-4xl font-semibold tracking-tight">
        Hello, flamingo-armond 🦩 — backend says: <code>{data.health}</code>
      </h1>
    </main>
  );
}
