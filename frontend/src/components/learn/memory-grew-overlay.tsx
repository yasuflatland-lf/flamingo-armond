"use client";

import { useEffect } from "react";
import { stabilityToPercent } from "./fsrs-stability";

export function MemoryGrewOverlay({
	from,
	to,
	onDone,
}: {
	from: number;
	to: number;
	onDone: () => void;
}) {
	const delta = stabilityToPercent(to) - stabilityToPercent(from);

	useEffect(() => {
		if (delta <= 0) {
			onDone();
			return;
		}
		const id = setTimeout(onDone, 1500);
		return () => clearTimeout(id);
	}, [from, to, onDone, delta]);

	if (delta <= 0) {
		return null;
	}

	return (
		<div className="pointer-events-none absolute inset-0 z-30 flex items-center justify-center">
			<div className="animate-bounce rounded-2xl bg-emerald-500/90 px-8 py-4 text-2xl font-bold text-white shadow-lg">
				Memory +{delta}%
			</div>
		</div>
	);
}
