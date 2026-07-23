import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Terms of Service",
  description: "The terms governing your use of trqsh.",
};

export default function TermsPage() {
  return (
    <div className="mx-auto max-w-3xl px-4 py-16 sm:px-6">
      <article className="prose">
        <h1>Terms of Service</h1>
        <p className="text-sm text-muted">Last updated: {new Date().getFullYear()}. This is a template — replace with counsel-reviewed terms before launch.</p>

        <h2>1. Agreement</h2>
        <p>
          By creating a trqsh account or using the trqsh agent, edge, or dashboard (the &quot;Service&quot;),
          you agree to these Terms. If you use the Service on behalf of an organization, you accept
          these Terms for that organization.
        </p>

        <h2>2. Acceptable use</h2>
        <p>
          You may not use trqsh to host phishing, malware, or other illegal content, to send spam, or
          to violate others&apos; rights. Public hostnames are subject to abuse screening, and we may
          suspend tunnels that threaten the Service or its users. See our{" "}
          <a href="/docs/security">security &amp; abuse policy</a>.
        </p>

        <h2>3. Accounts &amp; API keys</h2>
        <p>
          You are responsible for activity under your account and for keeping your API keys secret.
          Keys are shown once and can be revoked at any time from the dashboard.
        </p>

        <h2>4. Plans &amp; billing</h2>
        <p>
          Paid plans are billed through Stripe on the cadence you choose. Quotas and limits are those
          published on the <a href="/pricing">pricing page</a>. You can change or cancel your plan at
          any time; changes take effect as described there.
        </p>

        <h2>5. The open-source agent</h2>
        <p>
          The trqsh agent is open source under its published license. These Terms govern the hosted
          Service; your use of the agent&apos;s source is additionally governed by that license.
        </p>

        <h2>6. Warranty &amp; liability</h2>
        <p>
          The Service is provided &quot;as is&quot; without warranties of any kind. To the maximum
          extent permitted by law, trqsh is not liable for indirect or consequential damages.
        </p>

        <h2>7. Changes</h2>
        <p>We may update these Terms; material changes will be announced. Continued use constitutes acceptance.</p>

        <h2>Contact</h2>
        <p>Questions? Email legal@trqsh.uz.</p>
      </article>
    </div>
  );
}
