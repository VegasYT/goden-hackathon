import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import * as z from 'zod/mini';

import { openEventStream, pointPath } from '@shared/api';
import type { ConnectionState } from '@shared/ui/connection-status';

import { refreshAfterSync, refreshCategories } from './point-signals';

const offlineAfterMs = 5000;

const minuteOf = (date: Date) => Math.floor(date.getTime() / 60_000);

const updateSchema = z.object({ categories: z.array(z.string()) });

function categoriesOf(data: string): string[] {
	try {
		return updateSchema.parse(JSON.parse(data)).categories;
	} catch {
		return [];
	}
}

/**
 * Держит поток сигналов открытой точки и перечитывает по ним её данные. Возвращает состояние
 * связи для оболочки; `undefined`, пока поток ни разу не подключился.
 */
export function usePointEvents(
	pointId: string | undefined
): ConnectionState | undefined {
	const queryClient = useQueryClient();
	const [connection, setConnection] = useState<{
		pointId: string;
		state: ConnectionState;
	}>();

	useEffect(() => {
		if (!pointId) {
			return;
		}

		let synced = false;
		// Пока связи нет, здесь время обрыва, а до первого подключения время открытия точки.
		let droppedAt: Date | undefined;
		let offlineTimer: number | undefined;

		const markOnline = () => {
			const now = new Date();

			window.clearTimeout(offlineTimer);
			droppedAt = undefined;
			// Индикатор показывает время с точностью до минуты: сигнал в ту же минуту оболочку не перерисовывает.
			setConnection((previous) =>
				previous?.pointId === pointId &&
				previous.state.status === 'online' &&
				minuteOf(previous.state.updatedAt) === minuteOf(now)
					? previous
					: { pointId, state: { status: 'online', updatedAt: now } }
			);
		};
		const markDropped = () => {
			// Пакет сообщает о каждой неудачной попытке, а время обрыва нужно первое.
			if (droppedAt) {
				return;
			}

			const dataAt = new Date();

			droppedAt = dataAt;
			offlineTimer = window.setTimeout(() => {
				setConnection({
					pointId,
					state: { status: 'offline', dataAt, onRetry: stream.reconnect },
				});
			}, offlineAfterMs);
		};

		const stream = openEventStream(`${pointPath(pointId)}/events`, {
			onOpen: markOnline,
			onDrop: markDropped,
			events: {
				sync: () => {
					refreshAfterSync(queryClient, pointId, { reconnect: synced });
					synced = true;
				},
				update: (data) => {
					refreshCategories(queryClient, pointId, categoriesOf(data));
					markOnline();
				},
			},
		});

		markDropped();

		return () => {
			window.clearTimeout(offlineTimer);
			stream.close();
		};
	}, [pointId, queryClient]);

	// После смены точки состояние прежней не показывается, пока новый поток не сообщит своё.
	return connection && connection.pointId === pointId
		? connection.state
		: undefined;
}
