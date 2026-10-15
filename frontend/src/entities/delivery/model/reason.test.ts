import { describe, expect, it } from 'vitest';

import { reasonLabel } from './reason';

describe('reasonLabel', () => {
	it('известную причину называет по-русски', () => {
		expect(reasonLabel('transport_damage')).toBe('Повреждено при перевозке');
	});

	it.each(['пересчёт после разгрузки', 'constructor', 'toString'])(
		'свободный текст «%s» возвращает как есть',
		(reason) => {
			expect(reasonLabel(reason)).toBe(reason);
		}
	);
});
