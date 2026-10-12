import type { ComponentProps } from "react";
import { cn } from "../../lib/utils";

/** Yüklenirken içeriğin şeklini gösteren gri blok. Dekoratiftir; "yükleniyor" bilgisini çevresi verir. */
function Skeleton({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="skeleton" aria-hidden="true" className={cn("animate-pulse rounded-md bg-muted", className)} {...props} />;
}

export { Skeleton };
