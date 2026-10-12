import type { ComponentProps } from "react";
import { cn } from "../../lib/utils";

const field =
  "w-full min-w-0 rounded-md border border-input bg-card px-3 text-base shadow-xs transition-[color,box-shadow] outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/30 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive md:text-sm";

function Input({ className, ...props }: ComponentProps<"input">) {
  return <input data-slot="input" className={cn(field, "h-9 py-1", className)} {...props} />;
}

function Textarea({ className, ...props }: ComponentProps<"textarea">) {
  return <textarea data-slot="textarea" className={cn(field, "min-h-24 resize-y py-2", className)} {...props} />;
}

/** Tarayıcının kendi `<select>`'i, alanlarla aynı görünümde: klavye ve mobil seçici bedava gelir. */
function NativeSelect({ className, ...props }: ComponentProps<"select">) {
  return <select data-slot="select" className={cn(field, "h-9 w-auto py-1 pr-8", className)} {...props} />;
}

export { Input, NativeSelect, Textarea };
