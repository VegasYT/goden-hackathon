import {
	useMutation,
	useQuery,
	useQueryClient,
	type QueryClient,
} from '@tanstack/react-query';

import { dishKeys } from '@entities/dish';
import { stockKeys } from '@entities/stock';
import { ApiError } from '@shared/api';
import { useIdempotencyKey } from '@shared/lib/idempotency';
import { invalidateKeys } from '@shared/lib/query';

import { recommendationKeys } from './recommendations';
import { batchesQueryOptions, type Batch } from './stock';

interface BatchWriteOptions<T> {
	pointId: string;
	productId: string;
	/** Поле запроса, отказ по которому значит, что остаток партии уже меньше. */
	quantityField: 'quantity' | 'delta';
	write: (batchId: string, body: T, idempotencyKey: string) => Promise<Batch>;
	onDone: (after: Batch, body: T) => void;
}

/**
 * После записи в партию: партию обновляет ответ, перечитываются сводные данные склада, история
 * продукта и меню кухни, которое считается по тем же остаткам.
 */
function storeBatch(
	queryClient: QueryClient,
	pointId: string,
	productId: string,
	after: Batch
) {
	queryClient.setQueryData<Batch[]>(
		stockKeys.batches(pointId, productId),
		(batches) =>
			batches?.map((batch) => (batch.id === after.id ? after : batch))
	);

	invalidateKeys(queryClient, [
		stockKeys.lists(pointId),
		stockKeys.summaries(pointId),
		stockKeys.expirings(pointId),
		recommendationKeys.all(pointId),
		dishKeys.all(pointId),
	]);

	void queryClient.resetQueries({
		queryKey: stockKeys.movements(pointId, productId),
	});
}

/** Запись в партию из диалога: живые партии продукта, ключ повтора и разбор отказа сервера. */
export function useBatchWrite<T>({
	pointId,
	productId,
	quantityField,
	write,
	onDone,
}: BatchWriteOptions<T>) {
	const queryClient = useQueryClient();
	const batches = useQuery(batchesQueryOptions(pointId, productId));
	const keyFor = useIdempotencyKey();
	const mutation = useMutation({
		mutationFn: ({ batchId, body }: { batchId: string; body: T }) =>
			write(batchId, body, keyFor({ batchId, body })),
		onSuccess: (after, { body }) => {
			storeBatch(queryClient, pointId, productId, after);
			onDone(after, body);
		},
		onError: (error) => {
			// Сервер отказал по количеству: скорее всего, заказы уже израсходовали часть партии.
			if (error instanceof ApiError && error.fieldErrors[quantityField]) {
				void batches.refetch();
			}
		},
	});
	const fieldError =
		mutation.error instanceof ApiError
			? mutation.error.fieldErrors[quantityField]?.message
			: undefined;

	return {
		batches: batches.data,
		mutation,
		fieldError,
		otherError: fieldError ? undefined : mutation.error?.message,
	};
}
