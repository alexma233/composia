import { assertEquals } from "jsr:@std/assert@1.0.19";

import { dockerConnectHeaders, svelteKitRouteParam } from "./docker-route.ts";

Deno.test(
  "keeps already-decoded SvelteKit Docker route params unchanged",
  () => {
    assertEquals(svelteKitRouteParam("sha256%3Aabc"), "sha256%3Aabc");
  },
);

Deno.test(
  "forwards configured controller headers for Docker log streams",
  () => {
    assertEquals(dockerConnectHeaders("token", { "X-Proxy": "yes" }), {
      "X-Proxy": "yes",
      Authorization: "Bearer token",
      "Connect-Protocol-Version": "1",
      "Content-Type": "application/connect+json",
      "X-Composia-Source": "web",
    });
  },
);
