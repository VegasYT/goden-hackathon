import type { EventSourceInit, FetchLike } from 'eventsource';

/**
 * Подмена `EventSource` из пакета `eventsource` для тестов: в сеть не ходит, подключением управляет
 * тест. `vitest.setup.ts` ставит её вместо пакета во всех тестах.
 */
export class FakeEventSource extends EventTarget {
	static readonly CONNECTING = 0;
	static readonly OPEN = 1;
	static readonly CLOSED = 2;

	/** Все созданные подключения по порядку создания. */
	static instances: FakeEventSource[] = [];

	/** Последнее созданное подключение. */
	static get latest(): FakeEventSource {
		const latest = FakeEventSource.instances.at(-1);

		if (!latest) {
			throw new Error('Поток не открывали');
		}

		return latest;
	}

	/** Подключения, которые ещё не закрыты. */
	static active() {
		return FakeEventSource.instances.filter(
			(source) => source.readyState !== FakeEventSource.CLOSED
		);
	}

	static reset() {
		FakeEventSource.instances = [];
	}

	readonly url: string;
	/** `fetch` из опций: через него пакет отправил бы запрос. */
	readonly fetch: FetchLike | undefined;
	readyState = FakeEventSource.CONNECTING;

	constructor(url: string | URL, init?: EventSourceInit) {
		super();
		this.url = String(url);
		this.fetch = init?.fetch;
		FakeEventSource.instances.push(this);
	}

	open() {
		this.readyState = FakeEventSource.OPEN;
		this.dispatchEvent(new Event('open'));
	}

	/** Событие потока с данными в JSON. */
	emit(type: string, data: unknown) {
		this.dispatchEvent(new MessageEvent(type, { data: JSON.stringify(data) }));
	}

	/**
	 * Обрыв. По умолчанию пакет переподключается сам (`CONNECTING`), с `closed` — сдался, как при
	 * ответе не 200.
	 */
	fail({ closed = false } = {}) {
		this.readyState = closed
			? FakeEventSource.CLOSED
			: FakeEventSource.CONNECTING;
		this.dispatchEvent(new Event('error'));
	}

	close() {
		this.readyState = FakeEventSource.CLOSED;
	}
}
