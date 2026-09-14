import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { cn } from "cn";

type ServiceCardProps = {
  icon: React.ComponentType<{ className?: string; "aria-hidden"?: boolean }>;
  name: string;
  description: string;
  href: string;
  className?: string;
};

export function ServiceCard({
  icon: Icon,
  name,
  description,
  href,
  className,
}: ServiceCardProps) {
  return (
    <Link
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`Buka layanan ${name}`}
      className={cn(
        "group flex cursor-pointer flex-col gap-3 rounded-lg border border-border bg-card p-5",
        "transition-colors hover:border-ring/60 hover:bg-accent",
        "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
        className,
      )}
    >
      <div className="flex items-start justify-between">
        <span className="flex size-10 items-center justify-center rounded-md border border-border bg-background text-foreground">
          <Icon className="size-5" aria-hidden />
        </span>
        <ArrowUpRight
          className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5"
          aria-hidden
        />
      </div>
      <div className="space-y-1">
        <h3 className="font-medium text-foreground">{name}</h3>
        <p className="text-sm leading-relaxed text-muted-foreground">{description}</p>
      </div>
    </Link>
  );
}
