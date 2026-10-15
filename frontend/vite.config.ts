/// <reference types="vitest/config" />
import babel from '@rolldown/plugin-babel';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
import react, { reactCompilerPreset } from '@vitejs/plugin-react';
import { keycloakify } from 'keycloakify/vite-plugin/index.js';
import { defineConfig, loadEnv } from 'vite';
import svgr from 'vite-plugin-svgr';

import { colorModeInitScript } from './src/shared/ui/theme/color-mode-script.ts';

// https://vite.dev/config/
export default defineConfig(({ mode }) => ({
	plugins: [
		// Плагин роутера идёт до react(), чтобы генерация роутов работала.
		tanstackRouter({
			target: 'react',
			// В тестах страницы грузятся сразу: иначе первый тест каждого файла сам собирает всю страницу.
			autoCodeSplitting: mode !== 'test',
			routesDirectory: './src/app/routes',
			generatedRouteTree: './src/app/routeTree.gen.ts',
		}),
		react(),
		// Тема ставится до загрузки приложения, чтобы страница не мигала светлой.
		{
			name: 'theme-init',
			transformIndexHtml: () => [
				{
					tag: 'script',
					attrs: { id: 'theme-init' },
					children: colorModeInitScript,
					injectTo: 'head',
				},
			],
		},
		babel({ presets: [reactCompilerPreset()] }),
		// SVG как React-компонент только по суффиксу `?react`, обычный импорт `.svg` остаётся URL.
		svgr({ include: '**/*.svg?react', svgrOptions: { icon: true } }),
		// Тема страниц входа Keycloak; в тестах не нужна, а при запуске копирует ресурсы Keycloak в public.
		mode !== 'test' &&
			keycloakify({
				themeName: 'goden',
				accountThemeImplementation: 'none',
				keycloakVersionTargets: {
					'22-to-25': false,
					'all-other-versions': 'goden-keycloak-theme.jar',
				},
			}),
	],
	resolve: {
		tsconfigPaths: true,
	},
	server: {
		proxy: {
			'/api': {
				target:
					loadEnv(mode, process.cwd(), '').API_PROXY_TARGET ||
					'http://localhost:8080',
				// Удалённый стенд за HTTPS ждёт свой домен в Host и в SNI, а не localhost.
				changeOrigin: true,
			},
		},
	},
	test: {
		environment: 'jsdom',
		setupFiles: ['./vitest.setup.ts'],
		unstubGlobals: true,
		mockReset: true,
		unstubEnvs: true,
		// Сценарии через всё приложение при параллельном прогоне идут по несколько секунд, 5 секунд по умолчанию мало.
		testTimeout: 15_000,
	},
}));
