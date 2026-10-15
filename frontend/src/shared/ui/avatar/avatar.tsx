import { Box } from '@mui/material';

/** Две буквы имени: первые буквы двух слов или первые две буквы одного слова. */
function initials(name: string): string {
	const words = name.trim().split(/\s+/).filter(Boolean);
	const letters =
		words.length > 1 ? words[0][0] + words[1][0] : (words[0] ?? '').slice(0, 2);

	return letters.toUpperCase();
}

interface AvatarProps {
	name: string;
	size: 32 | 40 | 44;
	/** `brand` — вошедший пользователь, `neutral` — другие люди. */
	tone?: 'brand' | 'neutral';
}

/** Круг с инициалами человека; для программ чтения с экрана скрыт, имя пишется рядом. */
export function Avatar({ name, size, tone = 'brand' }: AvatarProps) {
	return (
		<Box
			component='span'
			aria-hidden
			sx={{
				display: 'flex',
				alignItems: 'center',
				justifyContent: 'center',
				flexShrink: 0,
				width: size,
				height: size,
				borderRadius: '50%',
				bgcolor: tone === 'brand' ? 'primary.tint' : 'track',
				color: tone === 'brand' ? 'primary.onTint' : 'text.primary',
				fontSize: size === 32 ? 12 : 13,
				fontWeight: 600,
			}}
		>
			{initials(name)}
		</Box>
	);
}
