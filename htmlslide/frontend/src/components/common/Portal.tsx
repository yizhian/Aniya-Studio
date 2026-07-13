import { useEffect, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";

/**
 * Renders children directly under `document.body`, escaping the local
 * React tree.
 *
 * This matters for global overlays (modals, drawers, previews): Framer
 * Motion keeps a persistent inline `transform` on any element it
 * animates (x/y/scale), and per the CSS spec, a `transform` on *any*
 * ancestor creates a new containing block + stacking context for
 * `position: fixed` descendants. Without a portal, a "fixed, high
 * z-index" overlay nested inside an animated ancestor can get silently
 * trapped beneath unrelated UI that never intended to sit above it.
 */
export function Portal({ children }: { children: ReactNode }) {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) return null;
  return createPortal(children, document.body);
}
