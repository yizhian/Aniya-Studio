interface Props {
  active: boolean;
  children: React.ReactNode;
}

export function BreathingBorder({ active, children }: Props) {
  return (
    <div className="relative rounded-2xl h-full">
      {active && (
        <div className="absolute inset-0 rounded-2xl pointer-events-none" style={{
          padding: "1.5px",
          background: "conic-gradient(from var(--angle, 0deg), transparent 0%, rgba(100,160,240,0.5) 25%, rgba(160,200,255,0.7) 50%, rgba(100,160,240,0.5) 75%, transparent 100%)",
          mask: "linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0)",
          maskComposite: "exclude", WebkitMaskComposite: "xor",
          animation: "breathing-rotate 3s linear infinite", zIndex: 5,
        }} />
      )}
      {active && (
        <div className="absolute inset-0 rounded-2xl pointer-events-none" style={{
          boxShadow: "inset 0 0 30px rgba(100,160,240,0.08)", zIndex: 4,
          animation: "breathing-pulse 2s ease-in-out infinite",
        }} />
      )}
      {children}
    </div>
  );
}
