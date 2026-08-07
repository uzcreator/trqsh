import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

// Same tone/token pairing as Badge (border-{tone}/30 bg-{tone}/10 text-{tone})
// so a banner and a pill always agree about what a given tone means.
const inlineAlertVariants = cva("flex items-start gap-2 rounded-md border px-3 py-2 text-xs", {
  variants: {
    tone: {
      neutral: "border-border bg-accent text-secondary",
      good: "border-good/30 bg-good/10 text-good",
      warning: "border-warning/30 bg-warning/10 text-warning",
      serious: "border-serious/30 bg-serious/10 text-serious",
      critical: "border-critical/30 bg-critical/10 text-critical",
      primary: "border-primary/30 bg-primary/10 text-primary",
    },
  },
  defaultVariants: { tone: "critical" },
});

export interface InlineAlertProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof inlineAlertVariants> {
  icon?: React.ComponentType<{ className?: string }>;
}

/** Inline banner for form/screen-level errors and warnings — replaces the
 *  error-banner markup that used to be copy-pasted at each call site. */
export function InlineAlert({ className, tone, icon: Icon, children, ...props }: InlineAlertProps) {
  return (
    <div className={cn(inlineAlertVariants({ tone }), className)} {...props}>
      {Icon && <Icon className="size-4 shrink-0" />}
      <span className="flex-1">{children}</span>
    </div>
  );
}

export { inlineAlertVariants };
