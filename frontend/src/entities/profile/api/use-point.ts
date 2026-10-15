import { useSuspenseQuery } from '@tanstack/react-query';

import { findPoint } from '../model/profile';
import { meQueryOptions, type Profile, type ProfilePoint } from './me-query';

/** Профиль и точка из адреса для страниц внутри точки: без точки в профиле `PointLayout` страницу не рисует. */
export function usePoint(pointId: string): {
	profile: Profile;
	point: ProfilePoint;
} {
	const { data: profile } = useSuspenseQuery(meQueryOptions);
	const point = findPoint(profile, pointId);

	if (!point) {
		throw new Error(`Точки ${pointId} нет в профиле`);
	}

	return { profile, point };
}
