"use client";

import { useActionState } from "react";
import { CheckCircle2, MonitorSmartphone } from "lucide-react";
import { approveDeviceAction, type ApproveState } from "./actions";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

/** The approve step for a signed-in user: confirm the code shown in the desktop
 *  app, which links the polling device to this account. */
export function DeviceApprove({ code }: { code: string }) {
  const [state, action, pending] = useActionState<ApproveState, FormData>(approveDeviceAction, {});

  if (state.ok) {
    return (
      <div className="flex flex-col items-center gap-3 text-center">
        <CheckCircle2 className="h-10 w-10 text-good" />
        <div>
          <p className="font-medium text-foreground">Device connected</p>
          <p className="mt-1 text-sm text-secondary">
            You can return to the trqsh app — it&apos;s signing you in now.
          </p>
        </div>
      </div>
    );
  }

  return (
    <form action={action} className="flex flex-col gap-4">
      <div className="flex items-center gap-3 rounded-md border border-border bg-accent/40 p-3 text-sm text-secondary">
        <MonitorSmartphone className="h-5 w-5 shrink-0 text-primary" />
        A device is asking to sign in to your trqsh account. Confirm the code it&apos;s showing.
      </div>
      <div className="flex flex-col gap-1.5">
        <label htmlFor="user_code" className="text-sm font-medium">
          Device code
        </label>
        <Input
          id="user_code"
          name="user_code"
          defaultValue={code}
          placeholder="WDJB-MJHT"
          autoComplete="off"
          autoCapitalize="characters"
          className="text-center font-mono text-lg tracking-widest"
          required
        />
      </div>
      {state.error && <p className="text-sm text-critical">{state.error}</p>}
      <Button type="submit" disabled={pending}>
        {pending ? "Approving…" : "Approve device"}
      </Button>
    </form>
  );
}
