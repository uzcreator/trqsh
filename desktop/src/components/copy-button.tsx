import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { Button, type ButtonProps } from "./ui/button";
import { cn } from "@/lib/utils";

async function writeClipboard(text: string) {
  // The WebView exposes the standard async clipboard in the secure app origin;
  // no native bridge needed.
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    /* clipboard blocked; ignore */
  }
}

/** Copies `value`, flashing a check for 1.2s. Icon-only by default. */
export function CopyButton({
  value,
  label,
  className,
  variant = "ghost",
  size = "icon",
  onCopied,
}: {
  value: string;
  label?: string;
  className?: string;
  variant?: ButtonProps["variant"];
  size?: ButtonProps["size"];
  onCopied?: () => void;
}) {
  const [copied, setCopied] = useState(false);
  return (
    <Button
      type="button"
      variant={variant}
      size={label ? "sm" : size}
      className={cn(className)}
      onClick={async () => {
        await writeClipboard(value);
        setCopied(true);
        onCopied?.();
        setTimeout(() => setCopied(false), 1200);
      }}
      title="Copy"
    >
      {copied ? <Check className="text-good" /> : <Copy />}
      {label && <span>{copied ? "Copied" : label}</span>}
    </Button>
  );
}
