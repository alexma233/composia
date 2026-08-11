import rawDeployConfig from "./.deno-deploy/deploy.json" with { type: "json" };
import rawSvelteData from "./.deno-deploy/svelte.json" with { type: "json" };
import { prepareServer } from "./.deno-deploy/handler.ts";
import { compressResponse, prepareCompressionRequest } from "./compression.mjs";

const handler = prepareServer(rawSvelteData, rawDeployConfig, Deno.cwd());

Deno.serve(
  {
    hostname: Deno.env.get("HOST") || "0.0.0.0",
    port: Number(Deno.env.get("PORT") || 3000),
  },
  async (request, info) =>
    compressResponse(
      request,
      await handler(prepareCompressionRequest(request), info),
    ),
);
