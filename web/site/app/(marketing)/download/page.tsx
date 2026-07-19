import type { Metadata } from "next";
import Link from "next/link";
import { ShieldCheck, TerminalSquare } from "lucide-react";
import { DownloadSection } from "@/components/download-section";
import { CodeBlock } from "@/components/code-block";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "Download",
  description:
    "Download the Rift desktop app for macOS, Windows, and Linux, or install the open-source CLI with Homebrew, Scoop, or a one-line script.",
};

export default function DownloadPage() {
  return (
    <div className="mx-auto max-w-content px-4 py-16 sm:px-6">
      <header className="mx-auto mb-12 max-w-2xl text-center">
        <h1 className="text-4xl font-semibold tracking-tight text-foreground">Download Rift</h1>
        <p className="mt-4 text-lg text-secondary">
          The desktop app or the open-source CLI — on every OS. Signed, notarized, and checksummed.
        </p>
      </header>

      <DownloadSection />

      <section className="mx-auto mt-16 grid max-w-4xl gap-5 sm:grid-cols-2">
        <div className="rounded-lg border border-border bg-surface p-6 shadow-sm">
          <TerminalSquare className="mb-3 h-6 w-6 text-primary" />
          <h2 className="text-base font-semibold text-foreground">One-line install (macOS / Linux)</h2>
          <p className="mb-3 mt-1 text-sm text-secondary">
            The fastest path — detects your OS and architecture, then installs the <code>rift</code> CLI.
          </p>
          <CodeBlock code={`curl -fsSL ${site.installShUrl} | sh`} prompt />
        </div>
        <div className="rounded-lg border border-border bg-surface p-6 shadow-sm">
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
        <Link href="/docs/quickstart" className="text-primary hover:underline">
          quickstart
        </Link>{" "}
        — you&apos;ll have a live public URL in under a minute.
      </p>
    </div>
  );
}
