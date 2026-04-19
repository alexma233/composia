import type { RequestHandler } from "./$types";

import { loadMaterialIconSvg } from "$lib/server/material-icon-theme";

export const GET: RequestHandler = async ({ params }) => {
  const svg = await loadMaterialIconSvg(params.name);
  if (!svg) {
    return new Response("Icon not found.", { status: 404 });
  }

  return new Response(svg, {
    headers: {
      "cache-control": "public, max-age=31536000, immutable",
      "content-type": "image/svg+xml; charset=utf-8",
    },
  });
};
