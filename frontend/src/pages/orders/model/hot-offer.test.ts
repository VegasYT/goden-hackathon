import { afterEach, describe, expect, it, vi } from 'vitest';

import { hideChange, pinChange, revertChange } from './hot-offer';

afterEach(() => {
	vi.restoreAllMocks();
});

describe('ручные горячие предложения', () => {
	it('скрытие передаёт часовой пояс браузера', () => {
		vi.spyOn(Intl.DateTimeFormat.prototype, 'resolvedOptions').mockReturnValue({
			timeZone: 'Europe/Moscow',
		} as Intl.ResolvedDateTimeFormatOptions);

		expect(hideChange()).toEqual({
			mode: 'hidden',
			time_zone: 'Europe/Moscow',
		});
	});

	it('комментарий закрепления обрезается', () => {
		expect(pinChange('  Блюдо дня ')).toEqual({
			mode: 'pinned',
			comment: 'Блюдо дня',
		});
	});

	it('отмена возвращает прежнюю настройку блюда', () => {
		const pinned = {
			dish: { id: 'soup', name: 'Тыквенный суп' },
			mode: 'pinned' as const,
			comment: 'Блюдо дня',
			set_by: { type: 'user' as const, id: 'ilya' },
			set_at: '2026-10-15T07:42:00Z',
		};

		expect(revertChange([pinned], 'soup')).toEqual({
			mode: 'pinned',
			comment: 'Блюдо дня',
		});
		expect(revertChange([pinned], 'omelette')).toEqual({ mode: 'auto' });
	});
});
