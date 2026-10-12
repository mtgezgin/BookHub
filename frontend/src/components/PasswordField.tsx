import { Eye, EyeOff } from "lucide-react";
import { useId, useState, type ComponentProps } from "react";
import { Input } from "./ui/input";

/**
 * Göster/gizle düğmeli parola alanı. Etiket alana `for`/`id` ile bağlı: düğmeyi <label>'ın İÇİNE
 * koyamayız, çünkü bir etiket ikinci bir etiketlenebilir öğe (düğme) içeremez.
 * Düğmenin adı sabit ("Parolayı göster"); durumu aria-pressed söyler.
 */
export function PasswordField({ label, hint, ...props }: { label: string; hint?: string } & Omit<ComponentProps<"input">, "type" | "id">) {
  const id = useId();
  const [visible, setVisible] = useState(false);
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
        {hint && <> <span className="font-normal text-muted-foreground">{hint}</span></>}
      </label>
      <div className="relative">
        <Input id={id} type={visible ? "text" : "password"} className="pr-10" {...props} />
        <button
          type="button"
          aria-label="Parolayı göster"
          aria-pressed={visible}
          onClick={() => setVisible((v) => !v)}
          className="absolute inset-y-0 right-0 grid w-10 place-items-center rounded-r-md text-muted-foreground outline-none hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          {visible ? <EyeOff aria-hidden="true" className="size-4" /> : <Eye aria-hidden="true" className="size-4" />}
        </button>
      </div>
    </div>
  );
}
