import { Badge } from "@/components/ui/badge";
import { type MasteryStage, masteryStage } from "./fsrs-state";

const LABEL: Record<MasteryStage, string> = {
  new: "New",
  learning: "Learning",
  learned: "Learned",
};

const VARIANT: Record<MasteryStage, "outline" | "secondary" | "default"> = {
  new: "outline",
  learning: "secondary",
  learned: "default",
};

export function MasteryBadge({ state }: { state: number }) {
  const stage = masteryStage(state);
  return (
    <Badge
      variant={VARIANT[stage]}
      aria-label={`Mastery stage: ${LABEL[stage]}`}
      data-stage={stage}
      className="absolute right-3 top-3 text-xs"
    >
      {LABEL[stage]}
    </Badge>
  );
}
