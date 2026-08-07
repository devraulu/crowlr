// @ts-check

import js from "@eslint/js";
import { defineConfig } from "eslint/config";
import tseslint from "typescript-eslint";
import prettier from "eslint-config-prettier";

export default defineConfig({
  files: ["**/*.{js,ts}"],
  extends: [
    prettier,
    js.configs.recommended,
    tseslint.configs.strict,
    tseslint.configs.stylistic,
  ],
});
