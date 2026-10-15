import { useMutation, useQueryClient } from '@tanstack/react-query';

import {
	createPoint,
	disableCompany,
	organizationKeys,
	renameCompany,
	storePoint,
	transferCompany,
} from '@entities/organization';
import { refreshProfile } from '@entities/profile';
import { useIdempotencyKey } from '@shared/lib/idempotency';

export function useCreatePoint(companyId: string) {
	const keyFor = useIdempotencyKey();
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: (name: string) =>
			createPoint(companyId, name, keyFor({ companyId, name })),
		onSuccess: async (point) => {
			storePoint(queryClient, point);
			await refreshProfile(queryClient);
		},
	});
}

export function useRenameCompany(companyId: string) {
	const keyFor = useIdempotencyKey();
	const queryClient = useQueryClient();

	// В точках компании её названия нет, поэтому перечитывается только профиль.
	return useMutation({
		mutationFn: (name: string) =>
			renameCompany(companyId, name, keyFor({ name })),
		onSuccess: () => refreshProfile(queryClient),
	});
}

export function useTransferCompany(companyId: string) {
	const keyFor = useIdempotencyKey();
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: (email: string) =>
			transferCompany(companyId, email, keyFor({ email })),
		onSuccess: () => refreshProfile(queryClient),
	});
}

export function useDisableCompany(companyId: string) {
	const keyFor = useIdempotencyKey();
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: () => disableCompany(companyId, keyFor({ disable: companyId })),
		onSuccess: async () => {
			// Отключение компании меняет все её точки, поэтому они перечитываются.
			void queryClient.invalidateQueries({
				queryKey: organizationKeys.companyPoints(companyId),
			});
			await refreshProfile(queryClient);
		},
	});
}
