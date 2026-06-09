import { build } from "esbuild";

await build({
  entryPoints: ["vgc/calc/calc_server.mjs"],
  bundle: true,
  platform: "node",
  format: "esm",
  outdir: "vgc/calc/dist",
});
