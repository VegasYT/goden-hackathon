import { readStorage, writeStorage } from '@shared/lib/storage';

const key = (userId: string) => `goden.last-point.${userId}`;

export function readLastPoint(userId: string): string | null {
	return readStorage(key(userId));
}

export function writeLastPoint(userId: string, pointId: string): void {
	writeStorage(key(userId), pointId);
}
