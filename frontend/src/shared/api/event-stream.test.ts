import type { EventSourceFetchInit } from 'eventsource';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { getAccessToken } from '@shared/auth';
import { FakeEventSource } from '@shared/lib/test-utils';

import { openEventStream, type EventStream } from './event-stream';

vi.mock('@shared/auth', () => ({ getAccessToken: vi.fn() }));

const init: EventSourceFetchInit = {
	signal: undefined,
	headers: { Accept: 'text/event-stream' },
	mode: 'cors',
	cache: 'no-store',
	redirect: 'follow',
};

let stream: EventStream;
const handlers = { events: {}, onOpen: vi.fn(), onDrop: vi.fn() };

beforeEach(() => {
	vi.useFakeTimers();
	vi.mocked(getAccessToken).mockResolvedValue('fresh-token');
	stream = openEventStream('/points/lenina/events', handlers);
});

afterEach(() => {
	stream.close();
});

/** Подменяет сеть ответом-потоком и отдаёт функцию, которая пишет байты в его тело. */
async function connectBody() {
	let push: (text: string) => void = () => undefined;
	const body = new ReadableStream<Uint8Array>({
		start(controller) {
			push = (text) => controller.enqueue(new TextEncoder().encode(text));
		},
	});

	vi.stubGlobal(
		'fetch',
		vi
			.fn()
			.mockResolvedValue(
				new Response(body, { headers: { 'content-type': 'text/event-stream' } })
			)
	);

	const response = await FakeEventSource.latest.fetch!(
		FakeEventSource.latest.url,
		init
	);
	const reader = (response.body as ReadableStream<Uint8Array>).getReader();

	return async (text: string) => {
		push(text);
		await reader.read();
	};
}

describe('openEventStream', () => {
	it('подключается к адресу API с действующим токеном на каждое подключение', async () => {
		const fetch = vi.fn<
			(url: string, request: RequestInit) => Promise<Response>
		>(() => Promise.resolve(new Response('')));

		vi.stubGlobal('fetch', fetch);
		await FakeEventSource.latest.fetch!(FakeEventSource.latest.url, init);
		vi.mocked(getAccessToken).mockResolvedValue('next-token');
		stream.reconnect();
		await FakeEventSource.latest.fetch!(FakeEventSource.latest.url, init);

		expect(FakeEventSource.latest.url).toBe('/api/v1/points/lenina/events');
		expect(
			fetch.mock.calls.map(
				([, request]) =>
					(request.headers as Record<string, string>).Authorization
			)
		).toEqual(['Bearer fresh-token', 'Bearer next-token']);
	});

	it('переподключается, если из потока больше 45 секунд не пришло ни байта', async () => {
		const write = await connectBody();

		FakeEventSource.latest.open();
		await vi.advanceTimersByTimeAsync(40_000);
		await write(': keepalive\n\n');
		await vi.advanceTimersByTimeAsync(35_000);

		expect(FakeEventSource.instances).toHaveLength(1);

		await vi.advanceTimersByTimeAsync(15_000);

		expect(FakeEventSource.instances).toHaveLength(2);
		expect(FakeEventSource.active()).toHaveLength(1);
		expect(handlers.onDrop).toHaveBeenCalledOnce();
	});

	it('после отказа сервера подключается снова с растущей паузой и сбрасывает её после подключения', () => {
		const reconnectsAfter = (ms: number) => {
			const before = FakeEventSource.instances.length;

			FakeEventSource.latest.fail({ closed: true });
			vi.advanceTimersByTime(ms - 1);
			const early = FakeEventSource.instances.length > before;

			vi.advanceTimersByTime(1);
			return !early && FakeEventSource.instances.length === before + 1;
		};

		expect(reconnectsAfter(3000)).toBe(true);
		expect(reconnectsAfter(6000)).toBe(true);
		expect(reconnectsAfter(12_000)).toBe(true);
		FakeEventSource.latest.open();
		expect(reconnectsAfter(3000)).toBe(true);
	});

	it('после обычного обрыва оставляет переподключение пакету', () => {
		FakeEventSource.latest.open();
		FakeEventSource.latest.fail();
		vi.advanceTimersByTime(30_000);

		expect(FakeEventSource.instances).toHaveLength(1);
		expect(handlers.onDrop).toHaveBeenCalledOnce();
	});
});
