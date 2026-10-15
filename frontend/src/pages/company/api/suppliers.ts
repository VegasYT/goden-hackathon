import {
	queryOptions,
	useMutation,
	useQueryClient,
} from '@tanstack/react-query';

import { apiClient, companyPath, getData, type components } from '@shared/api';

export type Supplier = components['schemas']['Supplier'];
export type NewSupplier = components['schemas']['NewSupplier'];

const suppliersKey = (companyId: string) =>
	['companies', companyId, 'suppliers'] as const;

const suppliersPath = (companyId: string) =>
	`${companyPath(companyId)}/suppliers`;

/** Поставщики компании по названию; только для владельца. Ключей в списке нет. */
export const suppliersQueryOptions = (companyId: string) =>
	queryOptions({
		queryKey: suppliersKey(companyId),
		queryFn: getData<Supplier[]>(suppliersPath(companyId)),
	});

/** Запись поставщика: ответ без ключа кладётся в список, ключ получает только вызывающий. */
function useSupplierWrite<T>(
	companyId: string,
	write: (input: T) => Promise<Supplier>
) {
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: write,
		onSuccess: (response) => {
			const saved = { ...response, key: undefined };

			queryClient.setQueryData<Supplier[]>(
				suppliersKey(companyId),
				(list = []) =>
					[...list.filter(({ id }) => id !== saved.id), saved].sort((a, b) =>
						a.name.localeCompare(b.name, 'ru')
					)
			);
		},
	});
}

export function useCreateSupplier(companyId: string) {
	return useSupplierWrite(
		companyId,
		async (supplier: NewSupplier) =>
			(await apiClient.post<Supplier>(suppliersPath(companyId), supplier)).data
	);
}

export function useUpdateSupplier(companyId: string, supplierId: string) {
	return useSupplierWrite(
		companyId,
		async (supplier: NewSupplier) =>
			(
				await apiClient.put<Supplier>(
					`${suppliersPath(companyId)}/${supplierId}`,
					supplier
				)
			).data
	);
}

/** Выпуск нового ключа: прежний сразу перестаёт действовать. */
export function useIssueSupplierKey(companyId: string) {
	return useSupplierWrite(
		companyId,
		async (supplierId: string) =>
			(
				await apiClient.post<Supplier>(
					`${suppliersPath(companyId)}/${supplierId}/key`
				)
			).data
	);
}

export function useRevokeSupplierKey(companyId: string) {
	return useSupplierWrite(
		companyId,
		async (supplierId: string) =>
			(
				await apiClient.delete<Supplier>(
					`${suppliersPath(companyId)}/${supplierId}/key`
				)
			).data
	);
}
