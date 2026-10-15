/** Вошедший пользователь по токену Keycloak; права на точках берутся из профиля. */
export interface Session {
	/** Идентификатор пользователя в Keycloak (`sub`). */
	userId: string;
	name: string;
}

export interface TokenClaims {
	sub?: string;
	name?: string;
	preferred_username?: string;
}

export function sessionFromToken(claims: TokenClaims): Session {
	return {
		userId: claims.sub ?? '',
		name: claims.name || claims.preferred_username || '',
	};
}
