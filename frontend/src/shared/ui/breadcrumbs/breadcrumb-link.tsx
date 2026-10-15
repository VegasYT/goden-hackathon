import { Link as MuiLink, type LinkProps as MuiLinkProps } from '@mui/material';
import { createLink } from '@tanstack/react-router';
import { forwardRef } from 'react';

/** Шаг пути со ссылкой роутера: `<BreadcrumbLink to='/points/$pointId/deliveries' params={…}>`. */
export const BreadcrumbLink = createLink(
	forwardRef<HTMLAnchorElement, MuiLinkProps>((props, ref) => (
		<MuiLink
			ref={ref}
			underline='hover'
			sx={{ color: 'text.secondary' }}
			{...props}
		/>
	))
);
