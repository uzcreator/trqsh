"use client";

import { useActionState } from "react";
import { Github, Waypoints } from "lucide-react";
import { authenticate, type LoginState } from "./actions";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const API = process.env.RIFT_API_URL || "http://localhost:8080";

export default function LoginPage() {
  const [state, action, pending] = useActionState<LoginState, FormData>(authenticate, {});

  return (
    <main className="flex min-h-screen items-center justify-center bg-page px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center text-center">
          <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <Waypoints className="h-6 w-6" />
          </div>
          <h1 className="text-xl font-semibold tracking-tight">Sign in to Rift</h1>
          <p className="mt-1 text-sm text-secondary">Manage your tunnels, domains, and billing.</p>
        </div>

        <div className="rounded-lg border border-border bg-surface p-6 shadow-sm">
          <form action={action} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="email">Email</Label>
              <Input id="email" name="email" type="email" placeholder="you@example.com" required autoFocus />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="name">Name (new accounts)</Label>
              <Input id="name" name="name" type="text" placeholder="Your name" />
            </div>
            {state.error && <p className="text-sm text-critical">{state.error}</p>}
            <Button type="submit" disabled={pending}>
              {pending ? "Signing in…" : "Continue"}
            </Button>
          </form>

          <div className="my-4 flex items-center gap-3 text-xs text-muted">
            <div className="h-px flex-1 bg-border" />
            or
            <div className="h-px flex-1 bg-border" />
          </div>

          <div className="flex flex-col gap-2">
            <a href={`${API}/v1/auth/oauth/github`}>
              <Button variant="outline" className="w-full" type="button">
                <Github className="h-4 w-4" />
                Continue with GitHub
              </Button>
            </a>
            <a href={`${API}/v1/auth/oauth/google`}>
              <Button variant="outline" className="w-full" type="button">
                Continue with Google
              </Button>
            </a>
          </div>
        </div>

        <p className="mt-4 text-center text-xs text-muted">
          Email sign-in uses the dev auth flow. OAuth requires the provider to be configured on the API.
        </p>
      </div>
    </main>
  );
}
