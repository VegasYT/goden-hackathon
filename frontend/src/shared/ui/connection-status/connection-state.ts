/** Состояние связи с сервером для индикатора и плашки. */
export type ConnectionState =
	| { status: 'online'; updatedAt: Date }
	| {
			status: 'offline';
			/** Время, на которое актуальны показанные данные. */
			dataAt: Date;
			onRetry: () => void;
	  };
