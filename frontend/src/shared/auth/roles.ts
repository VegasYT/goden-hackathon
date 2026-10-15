/** Роли сотрудника на точке в порядке старшинства. */
export const roles = ['manager', 'approver', 'cook', 'waiter'] as const;

export type Role = (typeof roles)[number];

export const roleLabels: Record<Role, string> = {
	manager: 'Управляющий',
	approver: 'Приёмщик',
	cook: 'Повар',
	waiter: 'Официант',
};
