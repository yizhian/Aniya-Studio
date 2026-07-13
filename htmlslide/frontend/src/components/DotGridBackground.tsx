import React, { useEffect, useRef } from "react";

type Props = {
  /** Dot glow color, e.g. "rgba(66, 133, 244, 0.45)" */
  glowColor?: string;
  /** Base dot color, e.g. "rgba(255,255,255,0.12)" or "rgba(0,0,0,0.08)" */
  dotColor?: string;
};

const CONFIG = {
  dotSize: 1.2,
  dotSpacing: 24,
  lightRadius: 300,
  ease: 0.06,
  maxOpacityBoost: 0.5,
  maxScaleBoost: 0.8,
};

export function DotGridBackground({
  glowColor = "rgba(100, 160, 240, 0.45)",
  dotColor = "rgba(255, 255, 255, 0.12)",
}: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const mouseRef = useRef({ x: -1000, y: -1000 });
  const lightRef = useRef({ x: -1000, y: -1000 });
  const rafRef = useRef<number>(0);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const parent = canvas.parentElement;
    if (!parent) return;

    const resize = () => {
      const rect = parent.getBoundingClientRect();
      canvas.width = rect.width;
      canvas.height = rect.height;
    };
    resize();

    const observer = new ResizeObserver(resize);
    observer.observe(parent);

    const onMove = (e: MouseEvent) => {
      mouseRef.current = { x: e.clientX, y: e.clientY };
    };
    window.addEventListener("mousemove", onMove);

    const animate = () => {
      const light = lightRef.current;
      const mouse = mouseRef.current;

      light.x += (mouse.x - light.x) * CONFIG.ease;
      light.y += (mouse.y - light.y) * CONFIG.ease;

      ctx.clearRect(0, 0, canvas.width, canvas.height);

      const { dotSize, dotSpacing, lightRadius, maxOpacityBoost, maxScaleBoost } = CONFIG;

      for (let x = 0; x < canvas.width; x += dotSpacing) {
        for (let y = 0; y < canvas.height; y += dotSpacing) {
          const dx = x - light.x;
          const dy = y - light.y;
          const dist = Math.sqrt(dx * dx + dy * dy);

          if (dist < lightRadius) {
            const t = 1 - Math.pow(dist / lightRadius, 2);
            const alpha = t * maxOpacityBoost;
            const size = dotSize + t * maxScaleBoost;
            ctx.beginPath();
            ctx.arc(x, y, size, 0, Math.PI * 2);
            ctx.fillStyle = glowColor.replace(/[\d.]+\)$/, `${alpha})`);
            ctx.fill();
          }

          // Always draw the base dot
          ctx.beginPath();
          ctx.arc(x, y, dotSize, 0, Math.PI * 2);
          ctx.fillStyle = dotColor;
          ctx.fill();
        }
      }

      rafRef.current = requestAnimationFrame(animate);
    };

    rafRef.current = requestAnimationFrame(animate);

    return () => {
      cancelAnimationFrame(rafRef.current);
      observer.disconnect();
      window.removeEventListener("mousemove", onMove);
    };
  }, [glowColor, dotColor]);

  return (
    <canvas
      ref={canvasRef}
      aria-hidden="true"
      className="absolute inset-0 w-full h-full pointer-events-none"
      style={{ zIndex: 0 }}
    />
  );
}
