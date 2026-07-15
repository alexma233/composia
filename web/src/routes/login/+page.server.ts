import type { Actions, PageServerLoad } from "./$types";

import { fail, redirect } from "@sveltejs/kit";

import {
  LoginRequestBodyTooLargeError,
  readLimitedLoginFormData,
  sanitizeLoginRedirect,
} from "$lib/server/login";
import {
  authConfig,
  authenticate,
  createSessionToken,
  sessionCookie,
} from "$lib/server/session";

export const load: PageServerLoad = async ({ url }) => {
  const config = authConfig();
  return {
    next: sanitizeLoginRedirect(url.searchParams.get("next")),
    ready: config.ready,
    error: config.ready ? null : config.reason,
  };
};

export const actions: Actions = {
  default: async ({ request, cookies, url }) => {
    const config = authConfig();
    if (!config.ready) {
      return fail(500, { invalid: false, error: config.reason });
    }

    let formData: FormData;
    try {
      formData = await readLimitedLoginFormData(request);
    } catch (error) {
      if (error instanceof LoginRequestBodyTooLargeError) {
        return fail(413, { invalid: false, error: null });
      }
      throw error;
    }

    const username = String(formData.get("username") ?? "");
    const password = String(formData.get("password") ?? "");
    const next = sanitizeLoginRedirect(
      String(formData.get("next") ?? url.searchParams.get("next") ?? "/"),
    );

    const user = await authenticate(username, password);
    if (!user) {
      return fail(401, {
        invalid: true,
        error: null,
      });
    }

    cookies.set(sessionCookie(), createSessionToken(user), {
      path: "/",
      httpOnly: true,
      sameSite: "lax",
      secure: url.protocol === "https:",
      maxAge: 60 * 60 * 24 * 14,
    });

    throw redirect(303, next);
  },
};
