/** Префикс ключей данных точки: от него строятся ключи разделов, и всё по точке сбрасывается им одним. */
export const pointKey = (pointId: string) => ['points', pointId] as const;
