import Link from "next/link";
import { Github } from "lucide-react";
import { Logo } from "./logo";
import { footerNav, site } from "@/lib/site";

export function SiteFooter() {
  return (
    <footer className="border-t border-border bg-surface">
      <div className="mx-auto max-w-content px-4 py-12 sm:px-6 sm:py-14">
        <div className="grid grid-cols-1 gap-10 lg:grid-cols-[1.4fr_2.6fr] lg:gap-16">
          {/* Brand */}
          <div className="max-w-sm">
            <Logo />
            <p className="mt-4 text-sm leading-relaxed text-secondary">
              Your localhost, live on the internet — over QUIC, with a free tier that doesn&apos;t
              get in your way.
            </p>
            <a
              href={site.githubUrl}
              target="_blank"
              rel="noreferrer"
              className="mt-5 inline-flex items-center gap-2 rounded-lg border border-border-strong bg-page px-3 py-2 text-sm text-secondary transition-colors hover:border-brand/40 hover:text-foreground"
            >
              <Github className="h-4 w-4" />
              Open-source agent
            </a>
          </div>

          {/* Link columns: 2×2 on phones, 4-across from sm up */}
          <nav className="grid grid-cols-2 gap-x-6 gap-y-8 sm:grid-cols-4" aria-label="Footer">
            {footerNav.map((col) => (
              <div key={col.title}>
                <h4 className="text-xs font-semibold uppercase tracking-wider text-muted">{col.title}</h4>
                <ul className="mt-4 flex flex-col gap-1">
                  {col.links.map((link) => {
                    const cls =
                      "-mx-2 block rounded-md px-2 py-1.5 text-sm text-secondary transition-colors hover:bg-accent/60 hover:text-foreground";
                    return (
                      <li key={link.label}>
                        {link.external ? (
                          <a href={link.href} target="_blank" rel="noreferrer" className={cls}>
                            {link.label}
                          </a>
                        ) : (
                          <Link href={link.href} className={cls}>
                            {link.label}
                          </Link>
                        )}
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}
          </nav>
        </div>

        <div className="mt-12 flex flex-col items-start justify-between gap-3 border-t border-border pt-6 text-xs text-muted sm:flex-row sm:items-center">
          <p>© {new Date().getFullYear()} trqsh — all rights reserved.</p>
          <p className="tabular">v{site.version} · Built for developers who ship fast.</p>
        </div>
      </div>
    </footer>
  );
}
