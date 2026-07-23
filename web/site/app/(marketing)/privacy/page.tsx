import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Privacy Policy",
  description: "How trqsh handles your data.",
};

export default function PrivacyPage() {
  return (
    <div className="mx-auto max-w-3xl px-4 py-16 sm:px-6">
      <article className="prose">
        <h1>Privacy Policy</h1>
        <p className="text-sm text-muted">Last updated: {new Date().getFullYear()}. This is a template — replace with a counsel-reviewed policy before launch.</p>

        <h2>What we collect</h2>
        <ul>
          <li>
            <strong>Account data</strong> — your email, name, and OAuth provider identity when you
            sign up.
          </li>
          <li>
            <strong>Usage metadata</strong> — tunnel counts, bandwidth, and request totals used to
            enforce quotas and bill metered plans. We do not sell your data.
          </li>
          <li>
            <strong>Billing data</strong> — handled by Stripe; we store only subscription status and
            identifiers, never full card numbers.
          </li>
        </ul>

        <h2>Tunnel traffic</h2>
        <p>
          The edge proxies your traffic to deliver the Service; request bodies are not stored beyond
          what the request inspector keeps for your plan&apos;s retention window. The local inspector
          runs on your machine and keeps captures on your machine.
        </p>

        <h2>Cookies</h2>
        <p>
          The dashboard uses a session cookie to keep you signed in. The marketing site stores your
          light/dark theme preference locally in your browser — no tracking cookie is required to
          browse.
        </p>

        <h2>Your choices</h2>
        <p>
          You can export or delete your account data from the dashboard or by emailing
          privacy@trqsh.uz. Revoking API keys immediately stops agents that use them.
        </p>

        <h2>Contact</h2>
        <p>Privacy questions? Email privacy@trqsh.uz.</p>
      </article>
    </div>
  );
}
