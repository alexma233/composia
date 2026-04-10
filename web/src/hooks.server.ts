import type { Handle } from "@sveltejs/kit";

import { json, redirect } from "@sveltejs/kit";

import { readSessionToken, sessionCookie } from "$lib/server/session";

const publicRoutes = new Set(["/login"]);

export const handle: Handle = async ({ event, resolve }) => {
  const user = readSessionToken(event.cookies.get(sessionCookie()));
  event.locals.user = user;

  const pathname = event.url.pathname;
  const isPublicRoute = isPublicPath(pathname);
  if (!user && !isPublicRoute) {
    if (isDocumentRequest(event)) {
      throw redirect(303, `/login?next=${encodeURIComponent(pathname + event.url.search)}`);
    }
    return json({ error: "Authentication required." }, { status: 401 });
  }

  if (user && pathname === "/login") {
    throw redirect(303, event.url.searchParams.get("next") || "/");
  }

  return resolve(event);
};

function isPublicPath(pathname: string) {
  if (publicRoutes.has(pathname)) {
    return true;
  }
  return pathname.startsWith("/favicon") || pathname.startsWith("/manifest");
}

function isDocumentRequest(event: Parameters<Handle>[0]["event"]) {
  const accept = event.request.headers.get("accept") ?? "";
  return event.request.method === "GET" && accept.includes("text/html");
}
