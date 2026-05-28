export const LONG_TERM_MEMORY_DAYS = 21;

export function stabilityToPercent(stability: number): number {
	if (!Number.isFinite(stability) || stability < 0) {
		return 0;
	}
	return Math.min(Math.round((stability / LONG_TERM_MEMORY_DAYS) * 100), 100);
}
