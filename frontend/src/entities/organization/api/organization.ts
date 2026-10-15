import { queryOptions } from '@tanstack/react-query';

import {
	apiClient,
	companyPath,
	getData,
	idempotencyHeader,
	pointPath,
	type components,
} from '@shared/api';
import type { Role } from '@shared/auth';
import { pointKey } from '@shared/lib/query';

import { sortStaff } from '../model/staff';

export type Company = components['schemas']['Company'];
export type Point = components['schemas']['Point'];
export type Staff = components['schemas']['Staff'];
export type MigomOnPoint = components['schemas']['MigomOnPoint'];

/** Ключи запросов организации: сотрудники и сама точка — под префиксом точки, точки компании — под компанией. */
export const organizationKeys = {
	staff: (pointId: string) => [...pointKey(pointId), 'staff'] as const,
	point: (pointId: string) => [...pointKey(pointId), 'point'] as const,
	migom: (pointId: string) => [...pointKey(pointId), 'migom'] as const,
	companyPoints: (companyId: string) =>
		['companies', companyId, 'points'] as const,
};

/** Сотрудники точки по ролям и именам. */
export const staffQueryOptions = (pointId: string) =>
	queryOptions({
		queryKey: organizationKeys.staff(pointId),
		queryFn: getData<Staff[]>(`${pointPath(pointId)}/staff`),
		select: sortStaff,
	});

/** Точка с чатом Telegram. */
export const pointQueryOptions = (pointId: string) =>
	queryOptions({
		queryKey: organizationKeys.point(pointId),
		queryFn: getData<Point>(pointPath(pointId)),
	});

/** Какое заведение сервиса доставки Мигом подключено к точке; только для владельца. */
export const migomQueryOptions = (pointId: string) =>
	queryOptions({
		queryKey: organizationKeys.migom(pointId),
		queryFn: getData<MigomOnPoint>(`${pointPath(pointId)}/migom`),
	});

/** Все точки компании, включая отключённые; только для владельца. */
export const companyPointsQueryOptions = (companyId: string) =>
	queryOptions({
		queryKey: organizationKeys.companyPoints(companyId),
		queryFn: getData<Point[]>(`${companyPath(companyId)}/points`),
	});

export async function assignStaff(
	pointId: string,
	staff: { email: string; role: Role },
	idempotencyKey: string
) {
	return (
		await apiClient.post<Staff>(
			`${pointPath(pointId)}/staff`,
			staff,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

export async function changeStaffRole(
	pointId: string,
	userId: string,
	role: Role,
	idempotencyKey: string
) {
	return (
		await apiClient.put<Staff>(
			`${pointPath(pointId)}/staff/${encodeURIComponent(userId)}`,
			{ role },
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

export async function removeStaff(
	pointId: string,
	userId: string,
	idempotencyKey: string
) {
	await apiClient.delete(
		`${pointPath(pointId)}/staff/${encodeURIComponent(userId)}`,
		idempotencyHeader(idempotencyKey)
	);
}

export async function renamePoint(
	pointId: string,
	name: string,
	idempotencyKey: string
) {
	return (
		await apiClient.patch<Point>(
			pointPath(pointId),
			{ name },
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

export async function disablePoint(pointId: string, idempotencyKey: string) {
	return (
		await apiClient.post<Point>(
			`${pointPath(pointId)}/disable`,
			undefined,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

/** Задаёт чат уведомлений точки; без номера чат снимается. */
export async function setTelegramChat(
	pointId: string,
	chatId: number | undefined,
	idempotencyKey: string
) {
	return (
		await apiClient.put<Point>(
			`${pointPath(pointId)}/telegram-chat`,
			chatId === undefined ? {} : { chat_id: chatId },
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

/** Подключает к точке заведение Мигома по номеру; `null` отключает Мигом. */
export async function setMigomOnPoint(
	pointId: string,
	outletId: number | null
) {
	return (
		await apiClient.put<MigomOnPoint>(`${pointPath(pointId)}/migom`, {
			outlet_id: outletId,
		})
	).data;
}

export async function createCompany(name: string, idempotencyKey: string) {
	return (
		await apiClient.post<Company>(
			'/companies',
			{ name },
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

export async function createPoint(
	companyId: string,
	name: string,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Point>(
			`${companyPath(companyId)}/points`,
			{ name },
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

export async function renameCompany(
	companyId: string,
	name: string,
	idempotencyKey: string
) {
	return (
		await apiClient.patch<Company>(
			companyPath(companyId),
			{ name },
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

export async function transferCompany(
	companyId: string,
	email: string,
	idempotencyKey: string
) {
	return (
		await apiClient.put<Company>(
			`${companyPath(companyId)}/owner`,
			{ email },
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

export async function disableCompany(
	companyId: string,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Company>(
			`${companyPath(companyId)}/disable`,
			undefined,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}
