import { FlamingoMark } from "@/components/brand/flamingo-mark";

export function BootSplash() {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-[#FF6F79]">
      <FlamingoMark aria-hidden className="size-24" />
    </div>
  );
}
