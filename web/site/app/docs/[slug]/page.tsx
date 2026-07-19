import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft, ArrowRight } from "lucide-react";
import { mdSlugs, loadDoc, docEntry } from "@/lib/docs";
import { DocsToc } from "@/components/docs-toc";

// Statically pre-render every Markdown-backed doc. Bespoke routes (api, errors)
// are excluded because they aren't in mdSlugs.
export function generateStaticParams() {
  return mdSlugs.map((slug) => ({ slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const entry = docEntry(slug);
  if (!entry) return {};
  return { title: entry.title, description: entry.description };
}

export default async function DocPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const entry = docEntry(slug);
  if (!entry || entry.type !== "md") notFound();

  const doc = loadDoc(slug);
  if (!doc) notFound();

  // Prev/next within the flat Markdown order.
  const idx = mdSlugs.indexOf(slug);
  const prev = idx > 0 ? docEntry(mdSlugs[idx - 1]) : undefined;
  const next = idx < mdSlugs.length - 1 ? docEntry(mdSlugs[idx + 1]) : undefined;

  return (
    <div className="grid gap-10 py-10 xl:grid-cols-[minmax(0,1fr)_13rem]">
      <div className="min-w-0">
        <article className="prose" dangerouslySetInnerHTML={{ __html: doc.html }} />

        <nav className="mt-12 flex items-stretch justify-between gap-4 border-t border-border pt-6">
          {prev ? (
            <Link
              href={`/docs/${prev.slug}`}
              className="group flex flex-1 flex-col rounded-lg border border-border bg-surface p-4 transition-colors hover:border-border-strong"
            >
              <span className="flex items-center gap-1 text-xs text-muted">
                <ArrowLeft className="h-3.5 w-3.5" /> Previous
              </span>
              <span className="mt-1 text-sm font-medium text-foreground">{prev.title}</span>
            </Link>
          ) : (
            <span className="flex-1" />
          )}
          {next ? (
            <Link
              href={`/docs/${next.slug}`}
              className="group flex flex-1 flex-col items-end rounded-lg border border-border bg-surface p-4 text-right transition-colors hover:border-border-strong"
            >
              <span className="flex items-center gap-1 text-xs text-muted">
                Next <ArrowRight className="h-3.5 w-3.5" />
              </span>
              <span className="mt-1 text-sm font-medium text-foreground">{next.title}</span>
            </Link>
          ) : (
            <span className="flex-1" />
          )}
        </nav>
      </div>

      <aside className="hidden xl:block">
        <DocsToc items={doc.toc} />
      </aside>
    </div>
  );
}
