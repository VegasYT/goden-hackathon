import { useMutation, useQueryClient } from '@tanstack/react-query';

import {
	disablePoint,
	organizationKeys,
	setMigomOnPoint,
	renamePoint,
	setTelegramChat,
	storePoint,
} from '@entities/organization';
import { refreshProfile } from '@entities/profile';
import { useIdempotencyKey } from '@shared/lib/idempotency';

export function useRenamePoint(pointId: string) {
	const keyFor = useIdempotencyKey();
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: (name: string) => renamePoint(pointId, name, keyFor({ name })),
		onSuccess: async (point) => {
			storePoint(queryClient, point);
			await refreshProfile(queryClient);
		},
	});
}

export function useSetTelegramChat(pointId: string) {
	const keyFor = useIdempotencyKey();
	const queryClient = useQueryClient();

	// Чата в профиле нет, поэтому профиль не перечитывается.
	return useMutation({
		mutationFn: (chatId: number | undefined) =>
			setTelegramChat(pointId, chatId, keyFor({ chatId })),
		onSuccess: (point) => storePoint(queryClient, point),
	});
}

export function useDisablePoint(pointId: string) {
	const keyFor = useIdempotencyKey();
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: () => disablePoint(pointId, keyFor({ disable: pointId })),
		onSuccess: async (point) => {
			storePoint(queryClient, point);
			await refreshProfile(queryClient);
		},
	});
}

/** Подключает Мигом к точке или отключает его; ответ сразу кладётся в кэш. */
export function useSetMigom(pointId: string) {
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: (outletId: number | null) => setMigomOnPoint(pointId, outletId),
		onSuccess: (migom) =>
			queryClient.setQueryData(organizationKeys.migom(pointId), migom),
	});
}
