"use client";

import { usePathname } from "next/navigation";

// Replays a soft entrance animation whenever the route changes. Keying the
// wrapper on the pathname remounts it on navigation, so content fades up each
// time — a light touch that makes the app feel responsive.
export function PageTransition({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  return (
    <div key={pathname} className="animate-fade-up">
      {children}
    </div>
  );
}
