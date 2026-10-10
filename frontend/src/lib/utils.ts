import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/**
 * Sınıf listelerini birleştirir. twMerge ÇAKIŞANLARDA sonuncuyu tutar: bileşenin varsayılanı
 * `px-4`, çağıran `px-2` verirse sonuç `px-2` olur (ikisi birden kalsaydı hangisinin kazanacağı
 * CSS dosyasındaki sıraya kalırdı).
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
