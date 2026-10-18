import type { LibraryStatus } from "../api/types";
import { libraryStatusLabel } from "../lib/format";
import { cn } from "../lib/utils";

/** Kapağın üstündeki kitaplık rozeti. Durum yalnız renkle değil METİNLE anlatılır (UX-02). */
export function LibraryBadge({ status, className }: { status: LibraryStatus; className?: string }) {
  return (
    <span className={cn("rounded-md bg-background/90 px-1.5 py-0.5 text-xs font-medium text-foreground shadow-sm", className)}>
      <span className="sr-only">Kitaplığında: </span>
      {libraryStatusLabel(status)}
    </span>
  );
}
