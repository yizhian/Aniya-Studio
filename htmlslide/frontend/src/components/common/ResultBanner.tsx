import { CheckCircle2, XCircle } from "lucide-react";
import { motion, AnimatePresence } from "motion/react";

interface Props {
  result: { ok: boolean; message: string } | null;
}

export function ResultBanner({ result }: Props) {
  return (
    <AnimatePresence>
      {result && (
        <motion.div
          initial={{ opacity: 0, y: -4 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0 }}
          className={`flex items-start gap-2 px-3 py-2 rounded-xl text-xs border ${
            result.ok
              ? "bg-green-500/5 border-green-400/20 text-green-400"
              : "bg-red-500/5 border-red-400/20 text-red-400"
          }`}
        >
          {result.ok ? (
            <CheckCircle2 size={13} className="shrink-0 mt-0.5" />
          ) : (
            <XCircle size={13} className="shrink-0 mt-0.5" />
          )}
          <span className="leading-relaxed">{result.message}</span>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
