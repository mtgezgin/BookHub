import { cn } from "../lib/utils";

/** Marka işareti: açık kitap. public/favicon.svg ile aynı çizim — ikisi birlikte değişir. */
export function BrandMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" aria-hidden="true" className={cn("size-8", className)}>
      <rect width="32" height="32" rx="8" className="fill-primary" />
      <path
        d="M16 10.5c-2.2-1.6-5-2.2-8-1.8v13c3-.4 5.8.2 8 1.8 2.2-1.6 5-2.2 8-1.8v-13c-3-.4-5.8.2-8 1.8Zm0 0v13"
        fill="none"
        strokeWidth="1.8"
        strokeLinejoin="round"
        className="stroke-primary-foreground"
      />
    </svg>
  );
}
