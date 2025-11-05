import js from '@eslint/js'
import tseslint from '@typescript-eslint/eslint-plugin'
import tsparser from '@typescript-eslint/parser'
import react from 'eslint-plugin-react'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import prettier from 'eslint-plugin-prettier'

export default [
  {
    ignores: ['dist', 'node_modules', 'build', '*.config.js']
  },
  js.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      parser: tsparser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
        ecmaFeatures: {
          jsx: true
        }
      },
      globals: {
        window: 'readonly',
        document: 'readonly',
        console: 'readonly',
        setTimeout: 'readonly',
        clearTimeout: 'readonly',
        setInterval: 'readonly',
        clearInterval: 'readonly',
        fetch: 'readonly',
        localStorage: 'readonly',
        sessionStorage: 'readonly'
      }
    },
    plugins: {
      '@typescript-eslint': tseslint,
      'react': react,
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
      'prettier': prettier
    },
    rules: {
      ...tseslint.configs.recommended.rules,
      ...react.configs.recommended.rules,
      ...reactHooks.configs.recommended.rules,

      // Prettier integration
      'prettier/prettier': 'error',

      // React rules
      'react/react-in-jsx-scope': 'off',
      'react/prop-types': 'off',
      // 该规则在 TS 项目中经常与 TS 的类型检查重复，关闭以避免误报
      'no-undef': 'off',

      // TypeScript rules
      // 放宽以下规则以避免在不改变功能的情况下大面积改动代码
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/explicit-module-boundary-types': 'off',
      '@typescript-eslint/no-unused-vars': 'off',

      // React Refresh
      'react-refresh/only-export-components': 'off',

      // General rules
      'no-console': 'off',
      'no-debugger': 'off',

      // 新版 react-hooks 推荐规则在本项目会造成大量误报，关闭以免影响开发体验
      'react-hooks/set-state-in-effect': 'off',
      'react-hooks/static-components': 'off',
      'react-hooks/preserve-manual-memoization': 'off',

      // 某些字符串中包含未转义字符用于展示，关闭以避免不必要的修改
      'react/no-unescaped-entities': 'off',

      // 可视情况关闭依赖数组校验（如需严格可改为 'warn'）
      'react-hooks/exhaustive-deps': 'off'
    },
    settings: {
      react: {
        version: 'detect'
      }
    }
  }
]
