import js from '@eslint/js';
import stylistic from '@stylistic/eslint-plugin';
import pluginQuery from '@tanstack/eslint-plugin-query';
import pluginRouter from '@tanstack/eslint-plugin-router';
import prettier from 'eslint-config-prettier/flat';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import { defineConfig, globalIgnores } from 'eslint/config';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export default defineConfig([
	globalIgnores([
		'dist',
		'src/app/routeTree.gen.ts',
		'src/shared/api/schema.gen.ts',
		'src/app/keycloak-theme/kc.gen.tsx',
		'dist_keycloak',
		'public/keycloakify-dev-resources',
	]),
	{
		files: ['**/*.{ts,tsx}'],
		extends: [
			js.configs.recommended,
			tseslint.configs.recommended,
			reactHooks.configs.flat.recommended,
			reactRefresh.configs.vite,
			pluginQuery.configs['flat/recommended'],
			pluginRouter.configs['flat/recommended'],
		],
		languageOptions: {
			ecmaVersion: 2023,
			globals: globals.browser,
		},
	},
	prettier,
	{
		// Пустые строки отделяют логические шаги: Prettier их не расставляет.
		files: ['**/*.{ts,tsx}'],
		plugins: { '@stylistic': stylistic },
		rules: {
			'@stylistic/padding-line-between-statements': [
				'error',
				{ blankLine: 'always', prev: 'block-like', next: '*' },
				{ blankLine: 'always', prev: ['const', 'let'], next: '*' },
				{ blankLine: 'any', prev: ['const', 'let'], next: ['const', 'let'] },
				{ blankLine: 'always', prev: '*', next: ['case', 'default'] },
			],
		},
	},
]);
