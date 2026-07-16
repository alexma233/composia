import type { Handle } from "@sveltejs/kit";

import { redirect } from "@sveltejs/kit";

import { readSessionToken, sessionCookie } from "$lib/server/session";

import { jsonApiError } from "$lib/server/controller-route";
import {
  loginRequestBodySizeStatus,
  sanitizeLoginRedirect,
} from "$lib/server/login";
import { withRequestSignal } from "$lib/server/request-context";
import { normalizeLocale } from "$lib/i18n/locales";

const publicRoutes = new Set(["/login"]);

export const handle: Handle = async ({ event, resolve }) => {
  const oversizedLoginStatus = loginRequestBodySizeStatus(event.request);
  if (oversizedLoginStatus) {
    return new Response(null, { status: oversizedLoginStatus });
  }

  const locale = normalizeLocale(event.cookies.get("composia.locale"));
  const user = readSessionToken(event.cookies.get(sessionCookie()));
  event.locals.user = user;

  const pathname = event.url.pathname;
  const isPublicRoute = isPublicPath(pathname);
  if (!user && !isPublicRoute) {
    if (isDocumentRequest(event)) {
      throw redirect(
        303,
        `/login?next=${encodeURIComponent(pathname + event.url.search)}`,
      );
    }
    return jsonApiError("AUTHENTICATION_REQUIRED", 401);
  }

  if (user && pathname === "/login") {
    throw redirect(
      303,
      sanitizeLoginRedirect(event.url.searchParams.get("next")),
    );
  }

  return withRequestSignal(event.request.signal, () =>
    resolve(event, {
      transformPageChunk: ({ html }) =>
        html.replace('<html lang="en"', `<html lang="${locale}"`),
    }),
  );
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
