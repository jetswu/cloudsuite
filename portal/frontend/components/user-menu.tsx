"use client";

import { LogOut, UserRound } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { ThemeToggle } from "@/components/theme-toggle";

type UserMenuProps = {
  name?: string | null;
  email?: string | null;
  signOutAction: () => Promise<void>;
};

export function UserMenu({ name, email, signOutAction }: UserMenuProps) {
  const displayName = name || email || "Pengguna";
  const initial = (displayName[0] || "?").toUpperCase();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="flex items-center gap-2 rounded-md outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
        aria-label="Menu akun"
      >
        <Avatar className="size-8">
          <AvatarFallback className="bg-primary text-primary-foreground text-sm">
            {initial}
          </AvatarFallback>
        </Avatar>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel className="flex flex-col">
          <span className="font-medium">{displayName}</span>
          {email ? (
            <span className="text-xs font-normal text-muted-foreground">{email}</span>
          ) : null}
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <button type="button" className="w-full cursor-pointer">
            <UserRound className="size-4" aria-hidden />
            Profil
          </button>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <ThemeToggle />
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <form action={signOutAction} className="w-full">
            <button
              type="submit"
              className="flex w-full cursor-pointer items-center gap-2 text-destructive"
            >
              <LogOut className="size-4" aria-hidden />
              Keluar
            </button>
          </form>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
