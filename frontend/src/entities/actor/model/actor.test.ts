import { describe, expect, it } from 'vitest';

import { actorName, isMigom } from './actor';

describe('actorName', () => {
	it.each([
		[{ type: 'service', id: 'worker' }, 'Система'],
		[{ type: 'service', id: 'goden-generator' }, 'Генератор'],
		[{ type: 'service', id: 'migom-integration' }, 'Мигом'],
		[{ type: 'service', id: 'goden-demo' }, 'автоматически'],
		[
			{
				type: 'service',
				id: 'supplier:0199e1c2-0000-7000-8000-000000000001',
				name: 'Овощебаза Юг',
			},
			'Овощебаза Юг',
		],
		[{ type: 'user', id: 'anna', name: 'Анна Котова' }, 'Анна Котова'],
		[{ type: 'user', id: 'ilya' }, 'ilya'],
	] as const)('автор %o — «%s»', (actor, label) => {
		expect(actorName(actor)).toBe(label);
	});
});

describe('isMigom', () => {
	it.each([
		[{ type: 'service', id: 'migom-integration' }, true],
		[{ type: 'service', id: 'goden-generator' }, false],
		[{ type: 'user', id: 'migom-integration' }, false],
	] as const)('автор %o — %s', (actor, migom) => {
		expect(isMigom(actor)).toBe(migom);
	});
});
