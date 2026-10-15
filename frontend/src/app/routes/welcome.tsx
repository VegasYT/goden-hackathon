import { createFileRoute, redirect } from '@tanstack/react-router';

import { meQueryOptions } from '@entities/profile';
import { WelcomePage } from '@pages/welcome';

export const Route = createFileRoute('/welcome')({
	beforeLoad: async ({ context }) => {
		const profile = await context.queryClient.ensureQueryData(meQueryOptions);

		if (profile.points.length > 0) {
			throw redirect({ to: '/' });
		}
	},
	component: WelcomePage,
});
