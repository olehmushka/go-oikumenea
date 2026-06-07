import { apiForward } from "@/lib/api/server";

/**
 * The Backend-for-Frontend proxy — the ONLY path from the browser to the go-oikumenea API.
 * It reads the server session, attaches the bearer (apiForward), and relays the request to
 * /<...path><?query>. The browser thus never holds a token and never hits :8443 directly,
 * so there is no CORS surface on the Go app (D-WebUI).
 */
async function handle(req: Request, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  const url = new URL(req.url);
  const target = `/${path.join("/")}`;

  const method = req.method;
  const body =
    method === "GET" || method === "HEAD" ? undefined : await req.arrayBuffer();

  const headers: Record<string, string> = {};
  const ct = req.headers.get("content-type");
  if (ct) headers["Content-Type"] = ct;

  const res = await apiForward(target, {
    method,
    search: url.search,
    headers,
    body,
  });

  // Relay status + JSON body (incl. the Conjure SerializableError envelope) unchanged.
  const text = await res.text();
  return new Response(text, {
    status: res.status,
    headers: {
      "Content-Type": res.headers.get("content-type") ?? "application/json",
    },
  });
}

export const GET = handle;
export const POST = handle;
export const PUT = handle;
export const PATCH = handle;
export const DELETE = handle;
