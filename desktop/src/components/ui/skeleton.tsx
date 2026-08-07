import { cn } from "@/lib/utils";

/** A pulsing placeholder block. Screens compose these into shapes that
 *  roughly match their real layout, so first-load data arriving doesn't
 *  cause a jarring jump — and so a zeroed-out fetch never gets mistaken
 *  for real "you have nothing" data. */
export function Skeleton({ className }: { className?: string }) {
  return <div className={cn("animate-pulse rounded-md bg-border/60", className)} aria-hidden />;
}
