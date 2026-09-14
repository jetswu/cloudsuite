"use client";

import { useEffect, useState } from "react";
import { Moon, Sun } from "lucide-react";

export function ThemeToggle() {
  const [isDark, setIsDark] = useState(true);

  useEffect(() => {
    setIsDark(document.documentElement.classList.contains("dark"));
  }, []);

  function toggleTheme() {
    const root = document.documentElement;
    const next = !root.classList.contains("dark");
    root.classList.toggle("dark", next);
    setIsDark(next);
    try {
      localStorage.setItem("theme", next ? "dark" : "light");
    } catch {
      /* abaikan — preferensi tetap jalan untuk sesi ini */
    }
  }

  return (
    <button
      type="button"
      onClick={toggleTheme}
      className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-foreground outline-none transition-colors hover:bg-accent focus-visible:bg-accent"
      aria-label="Ubah mode terang/gelap"
    >
      <Sun className="size-4 dark:hidden" aria-hidden />
      <Moon className="hidden size-4 dark:block" aria-hidden />
      <span>{isDark ? "Mode terang" : "Mode gelap"}</span>
    </button>
  );
}
