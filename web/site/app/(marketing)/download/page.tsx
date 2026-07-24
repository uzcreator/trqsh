import type { Metadata } from "next";
import Link from "next/link";
import { ShieldCheck, TerminalSquare } from "lucide-react";
import { DownloadSection } from "@/components/download-section";
import { SmartDownload } from "@/components/smart-download";
import { CodeBlock } from "@/components/code-block";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "Download",
  description:
    "Install the trqsh CLI on macOS, Windows, and Linux — a one-line script, npm, pip, .deb/.rpm, or a static binary. The site auto-detects your platform.",
};

export default function DownloadPage() {
  return (
    <div className="relative overflow-hidden">
      <div className="aurora opacity-30" aria-hidden />
      <div className="relative mx-auto max-w-content px-4 py-16 sm:px-6">
        <header className="mx-auto mb-10 max-w-2xl text-center">
          <h1 className="text-4xl font-semibold tracking-tight text-foreground sm:text-5xl">
            Download <span className="gradient-text">trqsh</span>
          </h1>
          <p className="mt-4 text-lg text-secondary">
            The open-source CLI on every OS — a one-line script, npm, pip, or a static binary.
            We&apos;ve already picked the right build for your machine.
          </p>
        </header>

        {/* Auto-detected pick */}
        <div className="mx-auto mb-14 max-w-3xl">
          <SmartDownload variant="card" />
        </div>

        <h2 className="mb-6 text-center text-sm font-semibold uppercase tracking-widest text-muted">
          All platforms &amp; package managers
        </h2>
        <DownloadSection />

        <section className="mx-auto mt-16 grid max-w-4xl gap-5 sm:grid-cols-2">
          <div className="border-gradient rounded-2xl border border-border bg-surface p-6 shadow-sm">
            <TerminalSquare className="mb-3 h-6 w-6 text-brand" />
            <h2 className="text-base font-semibold text-foreground">One-line install (macOS / Linux)</h2>
            <p className="mb-3 mt-1 text-sm text-secondary">
              The fastest path — detects your OS and architecture, then installs the <code>trqsh</code> CLI.
            </p>
            <CodeBlock code={`curl -fsSL ${site.installShUrl} | sh`} prompt />
          </div>
          <div className="border-gradient rounded-2xl border border-border bg-surface p-6 shadow-sm">
            <ShieldCheck className="mb-3 h-6 w-6 text-good" />
            <h2 className="text-base font-semibold text-foreground">Verify your download</h2>
            <p className="mb-3 mt-1 text-sm text-secondary">
              Every release ships a <code>checksums.txt</code>. Confirm the archive before you run it:
            </p>
            <CodeBlock code={`shasum -a 256 -c checksums.txt --ignore-missing`} prompt />
          </div>
        </section>

        <p className="mx-auto mt-10 max-w-2xl text-center text-sm text-muted">
          After installing, head to the{" "}
          <Link href="/docs/quickstart" className="text-brand hover:underline">
            quickstart
          </Link>{" "}
          — you&apos;ll have a live public URL in under a minute.
        </p>
      </div>
    </div>
  );
}
