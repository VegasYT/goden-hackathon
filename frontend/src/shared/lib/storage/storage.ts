// localStorage бывает недоступен в приватном режиме, поэтому ошибки чтения и записи глушим.

export function readStorage(key: string): string | null {
	try {
		return window.localStorage.getItem(key);
	} catch {
		return null;
	}
}

export function writeStorage(key: string, value: string): void {
	try {
		window.localStorage.setItem(key, value);
	} catch {
		// Значение просто не сохранится между сессиями.
	}
}
