import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: [
      "node_modules/**",
      "frontend/**",
      "mobile/**",
      "build/**",
      "dist/**",
      ".tmp/**",
      ".pnpm-store/**",
      "test-results/**",
      "playwright-report/**",
      "cfst-results/**",
      ".agents/**",
      ".codegraph/**",
      ".idea/**",
    ],
  },
  ...tseslint.configs.recommended,
  {
    files: ["playwright.config.ts", "tests/**/*.ts"],
    rules: {
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          argsIgnorePattern: "^_",
          caughtErrorsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
        },
      ],
    },
  },
);
