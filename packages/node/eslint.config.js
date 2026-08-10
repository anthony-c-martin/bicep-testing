const eslint = require('@eslint/js');
const tsParser = require('@typescript-eslint/parser');
const tsPlugin = require('@typescript-eslint/eslint-plugin');
const jest = require('eslint-plugin-jest');
const globals = require('globals');

module.exports = [
  {
    ignores: ['dist/**'],
  },
  eslint.configs.recommended,
  {
    files: ['**/*.ts'],
    languageOptions: {
      parser: tsParser,
      globals: globals.node,
    },
    plugins: {
      '@typescript-eslint': tsPlugin,
    },
    rules: tsPlugin.configs.recommended.rules,
  },
  {
    files: ['test/**/*.ts'],
    ...jest.configs['flat/recommended'],
  },
  {
    files: ['**/*.js', '**/*.mjs'],
    languageOptions: {
      globals: globals.node,
    },
  },
];