"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { QrCode } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

// qr.<base>'s bare landing page: reached when a phone can't scan the QR and
// the code was typed in by hand instead of arriving already in the URL
// (qr.trqsh.uz/CODE, which app/remote/[code] serves directly).
export default function RemoteLandingPage() {
  const router = useRouter();
  const [code, setCode] = useState("");

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const trimmed = code.trim();
    if (trimmed) router.push(`/remote/${encodeURIComponent(trimmed)}`);
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-page px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center text-center">
          <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <QrCode className="h-6 w-6" />
          </div>
          <h1 className="text-xl font-semibold tracking-tight">Remote control</h1>
          <p className="mt-1 text-sm text-secondary">Enter the code shown by /qr in your console.</p>
        </div>

        <form onSubmit={onSubmit} className="rounded-lg border border-border bg-surface p-6 shadow-sm">
          <div className="flex flex-col gap-1.5">
            <label htmlFor="pair_code" className="text-sm font-medium">
              Pairing code
            </label>
            <Input
              id="pair_code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="WDJB-MJHT-QP7K"
              autoComplete="off"
              autoCapitalize="characters"
              autoFocus
              className="text-center font-mono text-lg tracking-widest"
              required
            />
          </div>
          <Button type="submit" className="mt-4 w-full">
            Connect
          </Button>
        </form>

        <p className="mt-4 text-center text-xs text-muted">
          This connects to whatever console is showing the code — never enter one someone sent you.
        </p>
      </div>
    </main>
  );
}
