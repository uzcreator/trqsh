import type { Metadata } from "next";
import { RemoteViewer } from "./viewer";

export const metadata: Metadata = {
  title: "trqsh · Remote",
  description: "Live view and remote control of a paired trqsh console.",
};

// Reached at qr.<base>/<code> (middleware.ts rewrites the bare qr.<base> host
// here) or directly at app.<base>/remote/<code>. No auth: the code itself,
// baked into the CLI's QR/link, is the capability — see internal/api/remote.go.
export default async function RemotePairPage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  return <RemoteViewer code={code} />;
}
