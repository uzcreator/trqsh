import { DocsSidebar } from "@/components/docs-sidebar";
import { SiteFooter } from "@/components/site-footer";

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <div className="mx-auto max-w-content px-4 sm:px-6">
        <div className="lg:grid lg:grid-cols-[15rem_minmax(0,1fr)] lg:gap-10">
          {/* Desktop sidebar */}
          <aside className="sticky top-16 hidden max-h-[calc(100vh-4rem)] self-start overflow-y-auto py-10 lg:block">
            <DocsSidebar />
          </aside>

          {/* Mobile disclosure nav */}
          <details className="border-b border-border py-3 lg:hidden">
            <summary className="cursor-pointer list-none text-sm font-medium text-foreground">
              Documentation menu
            </summary>
            <div className="pt-4">
              <DocsSidebar />
            </div>
          </details>

          <div className="min-w-0">{children}</div>
        </div>
      </div>
      <SiteFooter />
    </>
  );
}
