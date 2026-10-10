import { Moon, Sun } from "lucide-react";
import { useTheme } from "../lib/theme";
import { Button } from "./ui/button";

/** Açık/koyu tema. Adı sabit ("Koyu tema"); açık mı kapalı mı aria-pressed söyler. */
export function ThemeToggle() {
  const [theme, setTheme] = useTheme();
  const dark = theme === "dark";
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label="Koyu tema"
      aria-pressed={dark}
      title={dark ? "Açık temaya geç" : "Koyu temaya geç"}
      onClick={() => setTheme(dark ? "light" : "dark")}
    >
      {dark ? <Sun aria-hidden="true" /> : <Moon aria-hidden="true" />}
    </Button>
  );
}
