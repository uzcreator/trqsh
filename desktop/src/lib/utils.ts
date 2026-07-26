import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

/** Merge Tailwind class lists, resolving conflicts left-to-right. */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
