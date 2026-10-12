import type { ComponentProps } from "react";
import { cn } from "../../lib/utils";

/** Alanı içine alan etiket: `<label>Başlık<input/></label>` → tıklanınca alan odaklanır, ekran okuyucu adı bilir. */
function Label({ className, ...props }: ComponentProps<"label">) {
  return <label data-slot="label" className={cn("flex flex-col gap-1.5 text-sm font-medium", className)} {...props} />;
}

export { Label };
