import { useMutation, useQueryClient } from '@tanstack/react-query';

import {
	assignStaff,
	changeStaffRole,
	organizationKeys,
	removeStaff,
	type Staff,
} from '@entities/organization';
import type { Role } from '@shared/auth';
import { useIdempotencyKey } from '@shared/lib/idempotency';

/** Обновляет список сотрудников ответом записи: сервер возвращает сотрудника целиком. */
function useStoreStaff(pointId: string) {
	const queryClient = useQueryClient();

	return (update: (staff: Staff[]) => Staff[]) =>
		queryClient.setQueryData<Staff[]>(
			organizationKeys.staff(pointId),
			(staff) => staff && update(staff)
		);
}

export function useAssignStaff(pointId: string) {
	const keyFor = useIdempotencyKey();
	const store = useStoreStaff(pointId);

	return useMutation({
		mutationFn: (staff: { email: string; role: Role }) =>
			assignStaff(pointId, staff, keyFor(staff)),
		onSuccess: (added) => {
			store((staff) => [...staff, added]);
		},
	});
}

export function useChangeStaffRole(pointId: string) {
	const keyFor = useIdempotencyKey();
	const store = useStoreStaff(pointId);

	return useMutation({
		mutationFn: ({ userId, role }: { userId: string; role: Role }) =>
			changeStaffRole(pointId, userId, role, keyFor({ userId, role })),
		onSuccess: (changed) => {
			store((staff) =>
				staff.map((item) => (item.user_id === changed.user_id ? changed : item))
			);
		},
	});
}

export function useRemoveStaff(pointId: string) {
	const keyFor = useIdempotencyKey();
	const store = useStoreStaff(pointId);

	return useMutation({
		mutationFn: (userId: string) =>
			removeStaff(pointId, userId, keyFor({ userId })),
		onSuccess: (_, userId) => {
			store((staff) => staff.filter((item) => item.user_id !== userId));
		},
	});
}
