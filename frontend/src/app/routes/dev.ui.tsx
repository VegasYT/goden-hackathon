import { createFileRoute, notFound } from '@tanstack/react-router';

import { DevUiPage } from '@pages/dev-ui';

// В production-сборке условие ложно, поэтому сборщик не включает витрину в бандл.
export const Route = createFileRoute('/dev/ui')({
	beforeLoad: () => {
		if (import.meta.env.PROD) {
			throw notFound();
		}
	},
	component: import.meta.env.DEV ? DevUiPage : () => null,
});
