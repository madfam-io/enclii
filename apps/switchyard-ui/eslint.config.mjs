import { defineConfig, globalIgnores } from 'eslint/config'
import nextVitals from 'eslint-config-next/core-web-vitals'
import nextTs from 'eslint-config-next/typescript'

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    rules: {
      // Keep as warnings for now - can be incrementally fixed
      "react-hooks/exhaustive-deps": "warn",
      "react/no-unescaped-entities": "off",
      "@next/next/no-img-element": "warn",

      // TypeScript rules - relax for existing codebase
      "@typescript-eslint/no-explicit-any": "warn",
      "@typescript-eslint/no-unused-vars": "warn",
      "@typescript-eslint/no-empty-object-type": "warn",

      // React rules - relax for existing patterns
      "react-hooks/set-state-in-effect": "warn",
      "react-hooks/rules-of-hooks": "warn",

      // General rules
      "prefer-const": "warn",
    },
  },
  globalIgnores([".next/**", "out/**", "build/**", "node_modules/**", "next-env.d.ts"]),
])

export default eslintConfig
