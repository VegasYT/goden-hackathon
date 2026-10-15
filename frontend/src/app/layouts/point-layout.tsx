import { useSuspenseQuery } from '@tanstack/react-query';
import { Outlet } from '@tanstack/react-router';
import { useEffect } from 'react';

import {
	findPoint,
	meQueryOptions,
	readLastPoint,
	startPoint,
	writeLastPoint,
} from '@entities/profile';
import { PointUnavailablePage } from '@pages/errors/point-unavailable';
import { useSession } from '@shared/auth';

import { usePointEvents } from '../realtime/use-point-events';
import { AppShell } from './app-shell';

/** Оболочка точки без разделов и страница «Точка недоступна». */
export function PointUnavailable() {
	const { data: profile } = useSuspenseQuery(meQueryOptions);
	const { userId } = useSession();

	return (
		<AppShell profile={profile}>
			<PointUnavailablePage
				target={startPoint(profile, readLastPoint(userId))}
			/>
		</AppShell>
	);
}

/** Оболочка выбранной точки; если точка пропала из перечитанного профиля, показывает «Точка недоступна». */
export function PointLayout({ pointId }: { pointId: string }) {
	const { data: profile } = useSuspenseQuery(meQueryOptions);
	const { userId } = useSession();
	const point = findPoint(profile, pointId);
	const connection = usePointEvents(point?.id);

	useEffect(() => {
		if (point) {
			writeLastPoint(userId, point.id);
		}
	}, [point, userId]);

	if (!point) {
		return <PointUnavailable />;
	}

	return (
		<AppShell profile={profile} point={point} connection={connection}>
			<Outlet />
		</AppShell>
	);
}
