/**
 * `placeholderData` для запросов точки с ключом от `pointKey(pointId)` или компании с ключом
 * `['companies', companyId, …]`: пока идёт новый запрос той же точки, видны прежние данные, а данные другой точки не показываются.
 */
export function keepPointData(pointId: string) {
	return <T>(
		previous: T | undefined,
		previousQuery: { queryKey: readonly unknown[] } | undefined
	) => (previousQuery?.queryKey[1] === pointId ? previous : undefined);
}
