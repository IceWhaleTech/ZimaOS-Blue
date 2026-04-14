import path from 'node:path'
import { fileURLToPath } from 'node:url'
import js from '@eslint/js'
import { FlatCompat } from '@eslint/eslintrc'
import tseslint from 'typescript-eslint'
import pluginVue from 'eslint-plugin-vue'
import vueParser from 'vue-eslint-parser'
import eslintConfigPrettier from 'eslint-config-prettier'
import globals from 'globals'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const compat = new FlatCompat({
  baseDirectory: __dirname,
})
const vueRecommendedRules = Object.assign(
  {},
  ...compat.extends('plugin:vue/vue3-recommended').map((config) => config.rules ?? {})
)

export default [
  js.configs.recommended,
  ...tseslint.configs.recommended,
  eslintConfigPrettier,
  {
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node,
        FrameRequestCallback: 'readonly',
      },
    },
  },
  {
    files: ['**/*.vue'],
    languageOptions: {
      parser: vueParser,
      globals: {
        defineProps: 'readonly',
        defineEmits: 'readonly',
        defineExpose: 'readonly',
        withDefaults: 'readonly',
        defineSlots: 'readonly',
        defineOptions: 'readonly',
        defineModel: 'readonly',
      },
      parserOptions: {
        parser: {
          js: 'espree',
          ts: '@typescript-eslint/parser',
          '<template>': '@typescript-eslint/parser',
        },
        ecmaVersion: 'latest',
        sourceType: 'module',
        extraFileExtensions: ['.vue'],
      },
    },
    plugins: {
      vue: pluginVue,
    },
    rules: {
      ...vueRecommendedRules,
      'vue/comment-directive': 'off',
      'vue/valid-define-props': 'off',
      'vue/valid-define-emits': 'off',
    },
  },
  {
    rules: {
      'vue/multi-word-component-names': 'off',
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': ['error', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
        caughtErrorsIgnorePattern: '^_'
      }],
    },
  },
  {
    files: ['src/__tests__/**/*.{js,jsx,ts,tsx,vue}', 'src/**/*.{test,spec}.{js,jsx,ts,tsx,vue}'],
    rules: {
      '@typescript-eslint/no-explicit-any': 'off',
      'no-useless-escape': 'off',
    },
  },
  {
    files: ['src/components/ProviderPoolSection.vue', 'src/components/settings/ApiProxySettings.vue'],
    rules: {
      '@typescript-eslint/ban-ts-comment': 'off',
    },
  },
  {
    ignores: ['dist/', 'node_modules/'],
  },
]
