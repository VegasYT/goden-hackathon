import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { playNavIconMotion } from './motion';

describe('playNavIconMotion', () => {
	afterEach(() => vi.useRealTimers());

	it('анимация доигрывает до конца после ухода указателя и не перезапускается повторным наведением', () => {
		vi.useFakeTimers();
		render(
			<a href='/' onMouseEnter={playNavIconMotion}>
				Склад
			</a>
		);
		const link = screen.getByRole('link');

		fireEvent.mouseEnter(link);
		fireEvent.mouseLeave(link);
		vi.advanceTimersByTime(500);
		fireEvent.mouseEnter(link);

		expect(link).toHaveAttribute('data-motion-playing');

		vi.advanceTimersByTime(400);

		expect(link).not.toHaveAttribute('data-motion-playing');
	});
});
