import { act } from '@testing-library/react';

import type { components } from '@shared/api';
import { FakeEventSource } from '@shared/lib/test-utils';

// Запись по всем категориям контракта: новая категория без строки здесь не скомпилируется.
const categories: Record<components['schemas']['RealtimeCategory'], true> = {
	catalog: true,
	deliveries: true,
	expiry: true,
	orders: true,
	recommendations: true,
	stock: true,
	'stop-list': true,
};

/** Все категории сигналов, которые сервер называет в `sync`. */
export const allCategories = Object.keys(categories);

/** Подключает поток и присылает `sync`, как сервер при каждом подключении. */
export function connect(source = FakeEventSource.latest) {
	act(() => {
		source.open();
		source.emit('sync', { categories: allCategories });
	});
}

/** Присылает сигнал об изменении данных точки по категориям. */
export function signal(categories: string[]) {
	act(() => {
		FakeEventSource.latest.emit('update', {
			id: crypto.randomUUID(),
			occurred_at: new Date().toISOString(),
			categories,
		});
	});
}
