import { roles, type Role } from '@shared/auth';

interface StaffMember {
	name: string;
	role: Role;
}

const collator = new Intl.Collator('ru');

/** Сотрудники по старшинству роли, внутри роли — по имени. */
export function sortStaff<T extends StaffMember>(staff: T[]): T[] {
	return staff.toSorted(
		(a, b) =>
			roles.indexOf(a.role) - roles.indexOf(b.role) ||
			collator.compare(a.name, b.name)
	);
}

/** Может ли пользователь с правом управлять сотрудниками назначить роль: управляющих назначает только владелец. */
export function mayAssignRole(owner: boolean, role: Role): boolean {
	return owner || role !== 'manager';
}

/** Может ли пользователь менять роль сотрудника и снимать его: управляющих меняет только владелец. */
export function mayManageStaff(owner: boolean, target: StaffMember): boolean {
	return mayAssignRole(owner, target.role);
}

/** Роли в порядке показа в выборе и может ли пользователь их назначить: недоступные — в конце. */
export function roleChoices(
	owner: boolean
): { role: Role; assignable: boolean }[] {
	return roles
		.map((role) => ({ role, assignable: mayAssignRole(owner, role) }))
		.sort((a, b) => Number(b.assignable) - Number(a.assignable));
}
