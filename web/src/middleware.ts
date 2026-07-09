import { auth } from "@/auth";

/**
 * Gate every route behind a session except the login page and the Auth.js endpoints.
 * Unauthenticated requests are bounced to /login (which starts the Keycloak flow).
 */
export default auth((req) => {
  const { pathname, origin } = req.nextUrl;
  const isPublic =
    pathname.startsWith("/login") || pathname.startsWith("/api/auth");
  // No session, or a session whose bearer can no longer be refreshed (RefreshTokenError) — both are
  // effectively unauthenticated, so bounce to /login instead of letting a server read 401-error. This
  // mirrors the client-side instant-logout on a 401 (lib/api/unauthorized).
  const dead = !req.auth || req.auth.error === "RefreshTokenError";
  if (dead && !isPublic) {
    return Response.redirect(new URL("/login", origin));
  }
});

export const config = {
  // Run on app routes; skip Next internals and static assets.
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
