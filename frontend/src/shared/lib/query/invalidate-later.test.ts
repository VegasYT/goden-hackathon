import { QueryClient } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { invalidateLater } from './invalidate-later';

const queryKey = ['points', 'lenina', 'orders', 'list'];

function setup() {
	const queryClient = new QueryClient();
	const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

	return { queryClient, invalidate };
}

beforeEach(() => {
	vi.useFakeTimers();
});

describe('invalidateLater', () => {
	it('три вызова за полсекунды дают одно перечитывание через секунду после последнего', () => {
		const { queryClient, invalidate } = setup();

		invalidateLater(queryClient, queryKey);
		vi.advanceTimersByTime(250);
		invalidateLater(queryClient, queryKey);
		vi.advanceTimersByTime(250);
		invalidateLater(queryClient, queryKey);

		vi.advanceTimersByTime(999);
		expect(invalidate).not.toHaveBeenCalled();

		vi.advanceTimersByTime(1);
		expect(invalidate).toHaveBeenCalledTimes(1);

		vi.advanceTimersByTime(5000);
		expect(invalidate).toHaveBeenCalledTimes(1);
	});

	it('непрерывная серия перечитывает не реже раза в 3 секунды и ещё раз после серии', () => {
		const { queryClient, invalidate } = setup();
		const firedAt: number[] = [];
		const start = Date.now();

		invalidate.mockImplementation(() => {
			firedAt.push(Date.now() - start);
			return Promise.resolve();
		});

		for (let elapsed = 0; elapsed < 10_000; elapsed += 500) {
			invalidateLater(queryClient, queryKey);
			vi.advanceTimersByTime(500);
		}

		const lastCall = 9500;
		const checkpoints = [
			0,
			...firedAt.filter((at) => at <= lastCall),
			lastCall,
		];
		const longestWait = Math.max(
			...checkpoints.slice(1).map((at, index) => at - checkpoints[index])
		);

		expect(longestWait).toBeLessThanOrEqual(3000);

		vi.advanceTimersByTime(1000);
		expect(firedAt.filter((at) => at > lastCall)).toHaveLength(1);
	});
});
