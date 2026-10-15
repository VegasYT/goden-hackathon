import { describe, expect, it } from 'vitest';

import { sessionFromToken } from './session';

describe('sessionFromToken', () => {
	it('берёт имя из name, а без него — логин', () => {
		expect(
			sessionFromToken({ name: 'Повар Тестовый', preferred_username: 'cook' })
				.name
		).toBe('Повар Тестовый');
		expect(sessionFromToken({ preferred_username: 'cook' }).name).toBe('cook');
	});
});
