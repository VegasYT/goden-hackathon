import type { components } from '@shared/api';

/** Кто выполнил действие: сотрудник или сервисная учётная запись. */
export type Actor = components['schemas']['Actor'];

/** Сервисные учётные записи, которые пишут в систему: фоновый процесс, генератор событий и сервис доставки Мигом. */
const serviceLabels: Record<string, string> = {
	worker: 'Система',
	'goden-generator': 'Генератор',
	'migom-integration': 'Мигом',
};

/** Кто выполнил действие: сотрудник по имени или логину, известная сервисная учётная запись или поставщик по названию. */
export function actorName(actor: Actor): string {
	if (actor.type === 'service') {
		return serviceLabels[actor.id] ?? actor.name ?? 'автоматически';
	}

	return actor.name ?? actor.id;
}

/** Действие выполнил сервис доставки Мигом. */
export function isMigom(actor: Actor): boolean {
	return actor.type === 'service' && actor.id === 'migom-integration';
}
