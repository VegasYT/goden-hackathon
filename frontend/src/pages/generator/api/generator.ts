import { queryOptions } from '@tanstack/react-query';

import { apiClient, getData, pointPath, type components } from '@shared/api';
import { pointKey } from '@shared/lib/query';

export type GeneratorSettings = components['schemas']['GeneratorSettings'];
export type GeneratorUpdate = Omit<GeneratorSettings, 'updated_at'>;

const generatorPath = (pointId: string) => `${pointPath(pointId)}/generator`;

export const generatorKey = (pointId: string) =>
	[...pointKey(pointId), 'generator'] as const;

/** Настройки генератора на точке; у точки без настроек сервер отдаёт остановленный генератор. */
export function generatorQueryOptions(pointId: string) {
	return queryOptions({
		queryKey: generatorKey(pointId),
		queryFn: getData<GeneratorSettings>(generatorPath(pointId)),
	});
}

export async function saveGenerator(pointId: string, body: GeneratorUpdate) {
	return (await apiClient.put<GeneratorSettings>(generatorPath(pointId), body))
		.data;
}
