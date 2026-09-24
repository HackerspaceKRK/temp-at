import { useEffect, useRef, useState } from "react";

export function useHasBeenInView<T extends Element>(enabled = true, delayMs = 500) {
  const ref = useRef<T>(null);
  const [ready, setReady] = useState(false);
  const [hasBeenInView, setHasBeenInView] = useState(false);

  useEffect(() => {
    if (ready || !enabled) return;
    const timeout = setTimeout(() => setReady(true), delayMs);
    return () => clearTimeout(timeout);
  }, [enabled, ready, delayMs]);

  useEffect(() => {
    if (hasBeenInView || !ready || !ref.current) return;
    const observer = new IntersectionObserver((entries) => {
      if (entries[0].isIntersecting) {
        setHasBeenInView(true);
        observer.disconnect();
      }
    });
    observer.observe(ref.current);
    return () => observer.disconnect();
  }, [hasBeenInView, ready]);

  return { ref, hasBeenInView };
}
