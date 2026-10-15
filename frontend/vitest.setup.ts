import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

import { FakeEventSource } from './src/shared/lib/test-utils/fake-event-source';

// Поток точки в тестах не ходит в сеть: подключением управляет сам тест.
vi.mock('eventsource', () => ({ EventSource: FakeEventSource }));

afterEach(() => {
	vi.useRealTimers();
	cleanup();
	window.localStorage.clear();
	FakeEventSource.reset();
});
