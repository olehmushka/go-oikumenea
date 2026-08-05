import { redirect } from "next/navigation";
import { signIn, auth } from "@/auth";
import { consoleProviders } from "@/lib/auth/providers";
import { tg } from "@/lib/messages";

/**
 * Login screen — starts the Authorization-Code flow against a configured IdP.
 *
 * The buttons are rendered from the env-driven provider registry (D-MultiIdPExamples), so enabling
 * Google or Entra alongside Keycloak is a `.env` change with no edit here. Providers federated INTO
 * Keycloak (GitHub and friends, Example A) deliberately do not appear as separate buttons: to this
 * console they are one Keycloak login, and the provider choice happens on Keycloak's own screen.
 */
export default async function LoginPage() {
  const session = await auth();
  if (session && !session.error) redirect("/");

  return (
    <main className="flex min-h-screen items-center justify-center p-6">
      <div className="card w-full max-w-sm p-8 text-center">
        <h1 className="text-xl font-semibold text-slate-900">go-oikumenea</h1>
        <p className="mt-1 text-sm text-slate-500">{tg("Admin console")}</p>

        {consoleProviders.length === 0 ? (
          // Misconfiguration, not an empty state: with no credentials in the environment there is no
          // way to sign in at all, so say so instead of rendering a dead card.
          <p className="mt-6 text-sm text-amber-700">
            {tg("No identity provider is configured. Set the provider credentials in the console environment.")}
          </p>
        ) : (
          <div className="mt-6 space-y-2">
            {consoleProviders.map((provider) => (
              <form
                key={provider.id}
                action={async () => {
                  "use server";
                  await signIn(provider.id, { redirectTo: "/" });
                }}
              >
                <button type="submit" className="btn-primary w-full py-2">
                  {tg("Sign in with")} {provider.label}
                </button>
              </form>
            ))}
          </div>
        )}

        <p className="mt-4 text-xs text-slate-400">
          {tg("You will be redirected to your identity provider.")}
        </p>
      </div>
    </main>
  );
}
