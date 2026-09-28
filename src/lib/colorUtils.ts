// OKLCH & Color conversion utilities for Canvas and Theme colors

export interface ColorSwatch {
	id: string;
	name: string;
	hex: string;
}

export interface ThemeHarmonicPalette {
	accentGroup: ColorSwatch[];
	complementGroup: ColorSwatch[];
	splitGroup: ColorSwatch[];
	allThemeSwatches: ColorSwatch[];
}

export const CANVAS_PRESET_PALETTE: ColorSwatch[] = [
	{ id: 'rose', name: 'Rose', hex: '#f43f5e' },
	{ id: 'coral', name: 'Coral', hex: '#fb7185' },
	{ id: 'orange', name: 'Orange', hex: '#f97316' },
	{ id: 'amber', name: 'Amber', hex: '#f59e0b' },
	{ id: 'lime', name: 'Lime', hex: '#84cc16' },
	{ id: 'emerald', name: 'Emerald', hex: '#10b981' },
	{ id: 'mint', name: 'Mint', hex: '#34d399' },
	{ id: 'teal', name: 'Teal', hex: '#06b6d4' },
	{ id: 'sky', name: 'Sky', hex: '#0ea5e9' },
	{ id: 'blue', name: 'Blue', hex: '#3b82f6' },
	{ id: 'indigo', name: 'Indigo', hex: '#6366f1' },
	{ id: 'purple', name: 'Purple', hex: '#a855f7' },
	{ id: 'fuchsia', name: 'Fuchsia', hex: '#d946ef' },
	{ id: 'pink', name: 'Pink', hex: '#ec4899' }
];

/**
 * Converts OKLCH (L: 0-1, C: 0-0.4, H: 0-360) to 6-digit sRGB Hex code #rrggbb
 */
export function oklchToHex(l: number, c: number, h: number): string {
	const hRad = (h * Math.PI) / 180;
	const a = c * Math.cos(hRad);
	const b = c * Math.sin(hRad);

	const l_ = l + 0.3963377774 * a + 0.2158037573 * b;
	const m_ = l - 0.1055613458 * a - 0.0638541728 * b;
	const s_ = l - 0.0894841775 * a - 1.2914855480 * b;

	const l_cubed = l_ * l_ * l_;
	const m_cubed = m_ * m_ * m_;
	const s_cubed = s_ * s_ * s_;

	const r_lin = +4.0767439362 * l_cubed - 3.3077115913 * m_cubed + 0.2309699292 * s_cubed;
	const g_lin = -1.2684380046 * l_cubed + 2.6097574011 * m_cubed - 0.3413193965 * s_cubed;
	const b_lin = -0.0041960863 * l_cubed - 0.7034186147 * m_cubed + 1.7076147010 * s_cubed;

	const toSrgb = (x: number) => {
		const clamped = Math.max(0, Math.min(1, x));
		return clamped <= 0.0031308
			? 12.92 * clamped
			: 1.055 * Math.pow(clamped, 1.0 / 2.4) - 0.055;
	};

	const r = Math.round(toSrgb(r_lin) * 255);
	const g = Math.round(toSrgb(g_lin) * 255);
	const b_val = Math.round(toSrgb(b_lin) * 255);

	const toHex = (n: number) => n.toString(16).padStart(2, '0');
	return `#${toHex(r)}${toHex(g)}${toHex(b_val)}`;
}

/**
 * Converts sRGB Hex to approximate OKLCH values
 */
export function hexToOklch(hex: string): { l: number; c: number; h: number } | null {
	let clean = hex.trim().replace(/^#/, '');
	if (clean.length === 3) {
		clean = clean
			.split('')
			.map((char) => char + char)
			.join('');
	}
	if (clean.length !== 6) return null;

	const r = parseInt(clean.substring(0, 2), 16) / 255;
	const g = parseInt(clean.substring(2, 4), 16) / 255;
	const b = parseInt(clean.substring(4, 6), 16) / 255;

	if (isNaN(r) || isNaN(g) || isNaN(b)) return null;

	const fromSrgb = (x: number) => {
		return x <= 0.04045 ? x / 12.92 : Math.pow((x + 0.055) / 1.055, 2.4);
	};

	const r_lin = fromSrgb(r);
	const g_lin = fromSrgb(g);
	const b_lin = fromSrgb(b);

	const l_ = Math.cbrt(0.4122214708 * r_lin + 0.5363325363 * g_lin + 0.0514459929 * b_lin);
	const m_ = Math.cbrt(0.2119034982 * r_lin + 0.6806995451 * g_lin + 0.1073969566 * b_lin);
	const s_ = Math.cbrt(0.0883024619 * r_lin + 0.2817188376 * g_lin + 0.6299787005 * b_lin);

	const l = 0.2104542553 * l_ + 0.7936177850 * m_ - 0.0040720468 * s_;
	const a = 1.9779984951 * l_ - 2.4285922050 * m_ + 0.4505937099 * s_;
	const b_val = 0.0259040371 * l_ + 0.7827717662 * m_ - 0.8086757660 * s_;

	const c = Math.sqrt(a * a + b_val * b_val);
	let h = (Math.atan2(b_val, a) * 180) / Math.PI;
	if (h < 0) h += 360;

	return { l, c, h };
}

/**
 * Extracts the base hue angle (0-360) from the active theme state.
 */
export function getThemeBaseHue(accentId: string, customHue: number, swatch: string): number {
	if (accentId === 'custom') {
		return customHue;
	}
	const oklch = hexToOklch(swatch);
	if (oklch) return Math.round(oklch.h);
	return 165; // Emerald fallback
}

/**
 * Generates harmonic theme-derived colors:
 * - Accent tints & shades
 * - Complementary tints & shades (180deg)
 * - Split-complementary tints & shades (adjacent to complementary, +/- 30deg from complement)
 * - Neutral tints and shades
 */
export function generateThemeHarmonicPalette(baseHue: number): ThemeHarmonicPalette {
	const norm = (deg: number) => ((Math.round(deg) % 360) + 360) % 360;
	const h = norm(baseHue);
	const hComp = norm(h + 180);
	const hSplit1 = norm(h + 150);
	const hSplit2 = norm(h + 210);

	const accentGroup: ColorSwatch[] = [
		{ id: 'accent-base', name: 'Accent Base', hex: oklchToHex(0.62, 0.22, h) },
		{ id: 'accent-tint', name: 'Accent Tint', hex: oklchToHex(0.78, 0.14, h) },
		{ id: 'accent-shade', name: 'Accent Shade', hex: oklchToHex(0.44, 0.20, h) },
		{ id: 'accent-muted', name: 'Accent Soft', hex: oklchToHex(0.60, 0.08, h) }
	];

	const complementGroup: ColorSwatch[] = [
		{ id: 'comp-base', name: 'Complement Base', hex: oklchToHex(0.62, 0.22, hComp) },
		{ id: 'comp-tint', name: 'Complement Tint', hex: oklchToHex(0.78, 0.14, hComp) },
		{ id: 'comp-shade', name: 'Complement Shade', hex: oklchToHex(0.44, 0.20, hComp) },
		{ id: 'comp-muted', name: 'Complement Soft', hex: oklchToHex(0.60, 0.08, hComp) }
	];

	const splitGroup: ColorSwatch[] = [
		{ id: 'split1-base', name: 'Split Comp 1', hex: oklchToHex(0.62, 0.20, hSplit1) },
		{ id: 'split1-tint', name: 'Split 1 Tint', hex: oklchToHex(0.76, 0.13, hSplit1) },
		{ id: 'split1-shade', name: 'Split 1 Shade', hex: oklchToHex(0.44, 0.18, hSplit1) },
		{ id: 'split2-base', name: 'Split Comp 2', hex: oklchToHex(0.62, 0.20, hSplit2) },
		{ id: 'split2-tint', name: 'Split 2 Tint', hex: oklchToHex(0.76, 0.13, hSplit2) },
		{ id: 'split2-shade', name: 'Split 2 Shade', hex: oklchToHex(0.44, 0.18, hSplit2) }
	];

	return {
		accentGroup,
		complementGroup,
		splitGroup,
		allThemeSwatches: [...accentGroup, ...complementGroup, ...splitGroup]
	};
}
