import { EventSource, type FetchLike } from 'eventsource';

import { getAccessToken } from '@shared/auth';

import { apiClient } from './client';

// Сервер шлёт keepalive раз в 15 секунд, поэтому три пропуска подряд значат, что связи нет.
const silenceCheckMs = 15_000;
const silenceLimitMs = 45_000;
const retryDelaysMs = [3000, 6000, 12_000, 24_000, 30_000];

export interface EventStreamHandlers {
	/** Обработчики событий по типу, получают поле `data` события. */
	events: Record<string, (data: string) => void>;
	onOpen: () => void;
	/** Связь оборвалась, поток переподключается сам. */
	onDrop: () => void;
}

export interface EventStream {
	close: () => void;
	/** Закрывает текущее подключение и сразу открывает новое. */
	reconnect: () => void;
}

/** Открывает поток SSE по адресу API с токеном доступа и держит его, пока не вызван `close`. */
export function openEventStream(
	path: string,
	handlers: EventStreamHandlers
): EventStream {
	const url = apiClient.getUri({ url: path });
	let source: EventSource | undefined;
	let lastActivityAt = Date.now();
	let failures = 0;
	let retryTimer: number | undefined;

	// Тело ответа идёт через отметку времени, чтобы сторож видел и keepalive-комментарии.
	const fetchWithToken: FetchLike = async (input, init) => {
		const token = await getAccessToken();
		const response = await fetch(input, {
			...init,
			headers: { ...init.headers, Authorization: `Bearer ${token}` },
		});
		const body = response.body?.pipeThrough(
			new TransformStream<Uint8Array, Uint8Array>({
				transform(chunk, controller) {
					lastActivityAt = Date.now();
					controller.enqueue(chunk);
				},
			})
		);

		return {
			body: body ?? null,
			url: response.url,
			status: response.status,
			redirected: response.redirected,
			headers: response.headers,
		};
	};

	const connect = () => {
		window.clearTimeout(retryTimer);
		source?.close();
		lastActivityAt = Date.now();

		const current = new EventSource(url, { fetch: fetchWithToken });

		source = current;
		current.addEventListener('open', () => {
			failures = 0;
			lastActivityAt = Date.now();
			handlers.onOpen();
		});
		current.addEventListener('error', () => {
			if (current !== source) {
				return;
			}

			handlers.onDrop();
			// Пакет сам переподключается после обрыва, но сдаётся, если сервер ответил не потоком.
			if (current.readyState === EventSource.CLOSED) {
				retryTimer = window.setTimeout(
					connect,
					retryDelaysMs[Math.min(failures++, retryDelaysMs.length - 1)]
				);
			}
		});
		for (const [type, handle] of Object.entries(handlers.events)) {
			current.addEventListener(type, (event) => {
				handle(String(event.data));
			});
		}
	};

	const watchdog = window.setInterval(() => {
		if (
			source?.readyState !== EventSource.CLOSED &&
			Date.now() - lastActivityAt > silenceLimitMs
		) {
			handlers.onDrop();
			connect();
		}
	}, silenceCheckMs);

	connect();

	return {
		close: () => {
			window.clearInterval(watchdog);
			window.clearTimeout(retryTimer);
			source?.close();
			source = undefined;
		},
		reconnect: () => {
			failures = 0;
			connect();
		},
	};
}
