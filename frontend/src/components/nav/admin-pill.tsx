import Link from "next/link";
import { ShieldCheck } from "lucide-react";

export default function AdminPill() {
  return (
    <Link
      href="/admin"
      aria-label="Admin area"
      className="inline-flex items-center gap-1.5 rounded-full bg-brand-tint px-3 py-1 text-sm text-brand-tint-foreground border border-brand-tint-border hover:bg-opacity-90 transition-opacity"
    >
      <ShieldCheck className="h-4 w-4" aria-hidden="true" />
      Admin
    </Link>
  );
}
