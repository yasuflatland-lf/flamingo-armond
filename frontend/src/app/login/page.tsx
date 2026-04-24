import { LoginButton } from "./login-button";

type SearchParams = Promise<{ error?: string }>;

export default async function LoginPage({ searchParams }: { searchParams: SearchParams }) {
  const { error } = await searchParams;
  return (
    <main className="flex min-h-screen items-center justify-center p-8">
      <div className="flex flex-col items-center gap-4">
        <h1 className="text-2xl font-semibold">Sign in</h1>
        {error ? <p className="text-sm text-destructive">Sign-in failed: {error}</p> : null}
        <LoginButton />
      </div>
    </main>
  );
}
