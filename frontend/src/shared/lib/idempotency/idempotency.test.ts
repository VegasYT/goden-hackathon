import { renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { useIdempotencyKey } from './idempotency';

describe('ключ идемпотентности', () => {
	it('тот же для повтора, новый для другого тела', () => {
		const { result } = renderHook(() => useIdempotencyKey());
		const keyFor = result.current;
		const first = keyFor({ enabled: false });

		expect(keyFor({ enabled: false })).toBe(first);
		expect(keyFor({ enabled: true })).not.toBe(first);
	});
});
