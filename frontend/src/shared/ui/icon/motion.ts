import { keyframes, type CSSObject } from '@mui/material/styles';
import type { MouseEvent } from 'react';

// Сдвиги в единицах рисунка: у иконок Material Symbols он 960 единиц, 90 единиц - около 2px при 20px.
const motions: Record<
	string,
	{ frames: ReturnType<typeof keyframes>; origin?: string }
> = {
	pop: { frames: keyframes`35% { transform: scale(0.7); }` },
	hop: { frames: keyframes`40% { transform: translateY(-90px); }` },
	drive: {
		frames: keyframes`
			25% { transform: translateX(110px); }
			55% { transform: translateX(-50px); }
			80% { transform: translateX(20px); }
		`,
	},
	draw: {
		frames: keyframes`from { transform: scaleX(0); }`,
		origin: 'left center',
	},
	rise: {
		frames: keyframes`from { transform: translateY(90px); opacity: 0; }`,
	},
	tilt: {
		frames: keyframes`
			30% { transform: rotate(-12deg); }
			65% { transform: rotate(9deg); }
		`,
	},
	flip: { frames: keyframes`to { transform: rotate(180deg); }` },
	grow: {
		frames: keyframes`from { transform: scaleY(0.15); }`,
		origin: 'center bottom',
	},
	blink: { frames: keyframes`50% { transform: scaleY(0.1); }` },
	glow: { frames: keyframes`50% { opacity: 0.15; }` },
	// У шестерёнки шесть зубцов: после поворота на 60° она выглядит как до него.
	spin: { frames: keyframes`to { transform: rotate(60deg); }` },
};

const durationMs = 600;
const stepDelayMs = 80;
const maxStep = 3;

// Пока атрибут стоит на пункте, анимация идёт; его снимает playNavIconMotion после последнего шага.
const playing = '&[data-motion-playing]';

const playRules: CSSObject = {
	'& svg': { overflow: 'visible' },
	'& [data-motion]': {
		transformBox: 'fill-box',
		transformOrigin: 'center',
	},
	...Object.fromEntries(
		Object.entries(motions).map(([name, { frames, origin }]) => [
			`${playing} [data-motion="${name}"]`,
			{
				animation: `${frames} ${durationMs}ms ease both`,
				...(origin && { transformOrigin: origin }),
			},
		])
	),
	...Object.fromEntries(
		Array.from({ length: maxStep }, (_, i) => i + 1).map((step) => [
			`${playing} [data-step="${step}"]`,
			{ animationDelay: `${step * stepDelayMs}ms` },
		])
	),
};

/** Стили пункта, в иконке которого части (`data-motion`) анимируются; запуск - `playNavIconMotion`. */
export const navIconMotion: CSSObject = {
	'@media (prefers-reduced-motion: no-preference)': playRules,
};

/** Обработчик наведения на пункт: анимация иконки доигрывается до конца, даже если указатель сразу ушёл. */
export function playNavIconMotion(event: MouseEvent<HTMLElement>) {
	const host = event.currentTarget;

	if ('motionPlaying' in host.dataset) {
		return;
	}

	host.dataset.motionPlaying = '';
	window.setTimeout(
		() => delete host.dataset.motionPlaying,
		durationMs + maxStep * stepDelayMs
	);
}
