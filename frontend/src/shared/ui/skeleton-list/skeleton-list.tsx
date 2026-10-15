import { Skeleton, Stack, type SxProps, type Theme } from '@mui/material';

interface SkeletonListProps {
	/** Что загружается, для программ чтения с экрана: «Загружаем меню». */
	label: string;
	count?: number;
	height: number;
	sx?: SxProps<Theme>;
}

/** Заготовка строк списка, пока он загружается. */
export function SkeletonList({
	label,
	count = 5,
	height,
	sx,
}: SkeletonListProps) {
	return (
		<Stack
			sx={[{ gap: 1.5 }, ...(Array.isArray(sx) ? sx : [sx])]}
			aria-label={label}
			role='status'
		>
			{Array.from({ length: count }, (_, index) => (
				<Skeleton key={index} variant='rounded' height={height} />
			))}
		</Stack>
	);
}
