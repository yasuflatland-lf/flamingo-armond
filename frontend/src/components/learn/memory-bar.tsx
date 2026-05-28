import { stabilityToPercent } from "./fsrs-stability";

export function MemoryBar({ stability }: { stability: number }) {
  const percent = stabilityToPercent(stability);

  return (
    <div
      role="progressbar"
      aria-valuenow={percent}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label="Memory strength"
      className="h-2 w-full overflow-hidden rounded-full bg-muted"
    >
      <div
        className="h-full rounded-full bg-primary transition-all"
        style={{ width: `${percent}%` }}
      />
    </div>
  );
}
