import { redirect } from "next/navigation";
import { signIn, auth } from "@/auth";

/** Login screen — starts the Keycloak Authorization-Code flow. */
export default async function LoginPage() {
  const session = await auth();
  if (session && !session.error) redirect("/");

  return (
    <main className="flex min-h-screen items-center justify-center p-6">
      <div className="card w-full max-w-sm p-8 text-center">
        <h1 className="text-xl font-semibold text-slate-900">go-oikumenea</h1>
        <p className="mt-1 text-sm text-slate-500">Admin console</p>
        <form
          className="mt-6"
          action={async () => {
            "use server";
            await signIn("keycloak", { redirectTo: "/" });
          }}
        >
          <button type="submit" className="btn-primary w-full py-2">
            Sign in with Keycloak
          </button>
        </form>
        <p className="mt-4 text-xs text-slate-400">
          You will be redirected to your identity provider.
        </p>
      </div>
    </main>
  );
}
