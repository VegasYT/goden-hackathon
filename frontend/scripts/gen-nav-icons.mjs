// Генерирует src/shared/ui/icon/nav-icons.gen.tsx: иконки разделов навигации из контуров Material Symbols
// Rounded, разбитых на части для анимации при наведении (движения - src/shared/ui/icon/motion.ts).
//
// Пересборка: pnpm icons:nav
// Номера контуров иконки: node scripts/gen-nav-icons.mjs --contours shelves-fill
// (габариты и знак площади: у дырки он противоположен внешнему контуру).
//
// Часть - список номеров контуров. Отдельной частью может быть только закрашенный контур вместе
// с вложенными в него; дырку нельзя отделить от её внешнего контура, иначе она закрасится.
// Движение 'all' - вся иконка целиком.
import { readFileSync, writeFileSync } from 'node:fs';

const source = 'node_modules/@material-symbols/svg-400/rounded/';
const target = 'src/shared/ui/icon/nav-icons.gen.tsx';

/** Имя компонента -> файл Material Symbols, части обычной и заливной версий. */
const spec = {
	Dashboard: [
		'dashboard',
		[
			['pop', [1, 4, 8]],
			['pop', [0, 6, 9]],
			['pop', [3, 7, 11]],
			['pop', [2, 5, 10]],
		],
		[
			['pop', [1]],
			['pop', [0]],
			['pop', [3]],
			['pop', [2]],
		],
	],
	Stock: [
		'shelves',
		[
			['hop', [3, 5]],
			['hop', [4, 6]],
		],
		[['hop', 'all']],
	],
	Deliveries: ['local_shipping', [['drive', 'all']], [['drive', 'all']]],
	Orders: [
		'receipt_long',
		[
			['draw', [2]],
			['pop', [4]],
			['draw', [3]],
			['pop', [5]],
		],
		[
			['draw', [2]],
			['pop', [4]],
			['draw', [3]],
			['pop', [5]],
		],
	],
	Kitchen: [
		'skillet',
		[
			['rise', [0]],
			['rise', [1]],
			['rise', [2]],
			['tilt', [3, 4, 5]],
		],
		[
			['rise', [1]],
			['rise', [2]],
			['rise', [3]],
			['tilt', [0]],
		],
	],
	StopList: ['block', [['flip', 'all']], [['flip', 'all']]],
	Analytics: [
		'bar_chart',
		[
			['grow', [2]],
			['grow', [1]],
			['grow', [0]],
		],
		[
			['grow', [2]],
			['grow', [1]],
			['grow', [0]],
		],
	],
	Generator: [
		'smart_toy',
		[
			['tilt', 'all'],
			['blink', [1]],
			['blink', [2]],
		],
		[['tilt', 'all']],
	],
	Staff: [
		'group',
		[
			['hop', [0, 4, 6, 2, 5, 7]],
			['hop', [1, 3]],
		],
		[
			['hop', [0, 2]],
			['hop', [1, 3]],
		],
	],
	PointSettings: ['settings', [['spin', 'all']], [['spin', 'all']]],
	Company: [
		'domain',
		[
			['glow', [10]],
			['glow', [11]],
		],
		[
			['glow', [10]],
			['glow', [11]],
		],
	],
	Catalog: [
		'menu_book',
		[
			['draw', [4]],
			['draw', [6]],
			['draw', [5]],
		],
		[
			['draw', [2]],
			['draw', [4]],
			['draw', [3]],
		],
	],
};

/** Движения, у которых части идут по очереди (data-step). */
const staggered = new Set(['pop', 'hop', 'draw', 'rise', 'grow', 'glow']);

const fmt = (v) => String(Math.round(v * 1000) / 1000);

/** Контуры пути в абсолютных командах и их опорные точки. */
function contours(d) {
	const tokens = d.match(/[MmLlHhVvQqTtZz]|-?(?:\d+\.?\d*|\.\d+)(?:e-?\d+)?/g);
	const out = [];
	let i = 0;
	let cmd = '';
	let x = 0;
	let y = 0;
	let sx = 0;
	let sy = 0;
	let qx = null;
	let qy = null;
	let cur = [];
	let pts = [];
	const num = () => Number(tokens[i++]);
	const flush = () => {
		if (cur.length) out.push({ d: cur.join(' '), pts });
		cur = [];
		pts = [];
	};

	while (i < tokens.length) {
		if (/[A-Za-z]/.test(tokens[i])) cmd = tokens[i++];
		const rel = cmd === cmd.toLowerCase();
		const c = cmd.toUpperCase();

		if (c === 'Z') {
			cur.push('Z');
			x = sx;
			y = sy;
			qx = qy = null;
			flush();
		} else if (c === 'M') {
			let nx = num();
			let ny = num();
			if (rel) [nx, ny] = [x + nx, y + ny];
			flush();
			x = sx = nx;
			y = sy = ny;
			cur.push(`M${fmt(x)} ${fmt(y)}`);
			pts.push([x, y]);
			qx = qy = null;
			cmd = rel ? 'l' : 'L';
		} else if (c === 'L') {
			let nx = num();
			let ny = num();
			if (rel) [nx, ny] = [x + nx, y + ny];
			[x, y] = [nx, ny];
			cur.push(`L${fmt(x)} ${fmt(y)}`);
			pts.push([x, y]);
			qx = qy = null;
		} else if (c === 'H') {
			const nx = num();
			x = rel ? x + nx : nx;
			cur.push(`H${fmt(x)}`);
			pts.push([x, y]);
			qx = qy = null;
		} else if (c === 'V') {
			const ny = num();
			y = rel ? y + ny : ny;
			cur.push(`V${fmt(y)}`);
			pts.push([x, y]);
			qx = qy = null;
		} else if (c === 'Q' || c === 'T') {
			let cx;
			let cy;
			if (c === 'Q') {
				cx = num();
				cy = num();
				if (rel) [cx, cy] = [x + cx, y + cy];
			} else {
				[cx, cy] = qx === null ? [x, y] : [2 * x - qx, 2 * y - qy];
			}
			let nx = num();
			let ny = num();
			if (rel) [nx, ny] = [x + nx, y + ny];
			cur.push(`Q${fmt(cx)} ${fmt(cy)} ${fmt(nx)} ${fmt(ny)}`);
			pts.push([cx, cy], [nx, ny]);
			[qx, qy, x, y] = [cx, cy, nx, ny];
		} else {
			throw new Error(`Команда пути ${cmd} не поддерживается`);
		}
	}
	flush();
	return out;
}

function iconContours(file) {
	const svg = readFileSync(`${source}${file}.svg`, 'utf8');
	return contours(svg.match(/ d="([^"]+)"/)[1]);
}

function paths(file, parts) {
	const cs = iconContours(file);
	const used = new Set();
	const out = [];
	let step = 0;
	for (const [motion, idx] of parts) {
		if (idx === 'all') continue;
		idx.forEach((n) => used.add(n));
		const stepAttr = staggered.has(motion) ? ` data-step='${step++}'` : '';
		out.push(
			`<path data-motion='${motion}'${stepAttr} d='${idx.map((n) => cs[n].d).join(' ')}' />`
		);
	}
	const base = cs.filter((_, n) => !used.has(n)).map((c) => c.d);
	return base.length ? [`<path d='${base.join(' ')}' />`, ...out] : out;
}

function component(name, file, parts) {
	const whole = parts.find(([, idx]) => idx === 'all')?.[0];
	const body = paths(file, parts).join('\n');
	const content = whole ? `<g data-motion='${whole}'>${body}</g>` : body;
	return `export function ${name}(props: SVGProps<SVGSVGElement>) {
	return (
		<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 -960 960 960' {...props}>
			${content}
		</svg>
	);
}
`;
}

if (process.argv[2] === '--contours') {
	iconContours(process.argv[3]).forEach(({ pts }, n) => {
		const xs = pts.map((p) => p[0]);
		const ys = pts.map((p) => p[1]);
		const area = pts.reduce((s, [x0, y0], k) => {
			const [x1, y1] = pts[(k + 1) % pts.length];
			return s + x0 * y1 - x1 * y0;
		}, 0);
		const box = [
			Math.min(...xs),
			Math.min(...ys),
			Math.max(...xs),
			Math.max(...ys),
		];
		console.log(n, area > 0 ? '+' : '-', box.map(Math.round).join(' '));
	});
} else {
	const components = Object.entries(spec).flatMap(
		([name, [file, outline, filled]]) => [
			component(`Nav${name}Outline`, file, outline),
			component(`Nav${name}Filled`, `${file}-fill`, filled),
		]
	);
	writeFileSync(
		target,
		`// Сгенерировано scripts/gen-nav-icons.mjs (pnpm icons:nav), руками не правим.
// Иконки разделов навигации: контуры Material Symbols Rounded, разбитые на части для анимации.
import type { SVGProps } from 'react';

${components.join('\n')}`
	);
}
