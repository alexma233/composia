import type { RequestHandler } from "./$types";

import { redirect } from "@sveltejs/kit";

import { sessionCookie } from "$lib/server/session";

export const POST: RequestHandler = async ({ cookies }) => {
  cookies.delete(sessionCookie(), { path: "/" });
  throw redirect(303, "/login");
};
