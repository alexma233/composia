import { defineConfig } from "vite";
import { sveltekit } from "@sveltejs/kit/vite";
import tailwindcss from "@tailwindcss/vite";

const codemirrorSingletons = [
  "@codemirror/state",
  "@codemirror/view",
  "@codemirror/language",
  "@lezer/common",
  "@lezer/highlight",
  "@lezer/lr",
];

export default defineConfig({
  resolve: {
    dedupe: codemirrorSingletons,
  },
  optimizeDeps: {
    exclude: [
      "codemirror",
      "@codemirror/commands",
      "@codemirror/language-data",
      "@codemirror/lint",
      ...codemirrorSingletons,
    ],
  },
  plugins: [tailwindcss(), sveltekit()],
});
