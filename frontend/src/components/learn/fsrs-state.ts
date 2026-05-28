export type MasteryStage = "new" | "learning" | "learned";

export function masteryStage(state: number): MasteryStage {
  switch (state) {
    case 1:
    case 3:
      return "learning";
    case 2:
      return "learned";
    default:
      return "new";
  }
}
