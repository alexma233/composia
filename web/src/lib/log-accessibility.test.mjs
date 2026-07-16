import { assertEquals } from "jsr:@std/assert@1.0.19/equals";

import { logLiveMode } from "./log-accessibility.ts";

Deno.test(
  "keeps buffered log announcements polite after stream completion",
  () => {
    assertEquals(logLiveMode("idle", false), "off");
    assertEquals(logLiveMode("streaming", false), "polite");
    assertEquals(logLiveMode("completed", true), "polite");
    assertEquals(logLiveMode("failed", true), "polite");
  },
);
