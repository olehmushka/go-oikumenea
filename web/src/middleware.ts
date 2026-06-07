import { auth } from "@/auth";

/**
 * Gate every route behind a session except the login page and the Auth.js endpoints.
 * Unauthenticated requests are bounced to /login (which starts the Keycloak flow).
 */
export default auth((req) => {
  const { pathname, origin } = req.nextUrl;
  const isPublic =
    pathname.startsWith("/login") || pathname.startsWith("/api/auth");
  if (!req.auth && !isPublic) {
    return Response.redirect(new URL("/login", origin));
  }
});

export const config = {
  // Run on app routes; skip Next internals and static assets.
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
