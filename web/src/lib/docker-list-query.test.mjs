import { assertEquals } from "jsr:@std/assert@1.0.19/equals";

import {
  buildDockerListPageUrl,
  dockerSearchDebounceMs,
} from "./docker-list-query.ts";

Deno.test(
  "Docker list URL builder keeps debounced search state shareable",
  () => {
    assertEquals(dockerSearchDebounceMs, 300);
    assertEquals(
      buildDockerListPageUrl(
        "/nodes/n1/docker/containers",
        {
          page: 1,
          search: "redis",
          sortBy: "name",
          sortDirection: "asc",
        },
        "name",
      ),
      "/nodes/n1/docker/containers?search=redis",
    );
  },
);
