import React, { useCallback, useEffect, useRef, useState } from "react";
import { motion } from "motion/react";
import { X, Maximize, Minimize } from "lucide-react";
import { useLocale } from "../../context/LocaleContext";
import { Portal } from "../common/Portal";
import { Z_INDEX } from "../../constants/zIndex";

type Props = {
  src: string;
  error: string | null;
  onClose: () => void;
};

export function EditorPreviewModal({ src, error, onClose }: Props) {
  const { t } = useLocale();
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(true);
  const [iframeReady, setIframeReady] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const hideTimerRef = useRef<ReturnType<typeof setTimeout>>();

  const toggleFullscreen = useCallback(() => {
    if (!containerRef.current) return;
    if (document.fullscreenElement) {
      document.exitFullscreen().catch(() => {});
    } else {
      containerRef.current.requestFullscreen().catch(() => {});
    }
  }, []);

  // Blur the fullscreen button and focus the iframe after entering fullscreen
  useEffect(() => {
    if (isFullscreen && iframeRef.current) {
      const timer = setTimeout(() => {
        iframeRef.current?.focus();
      }, 100);
      return () => clearTimeout(timer);
    }
  }, [isFullscreen]);

  useEffect(() => {
    const handleChange = () => {
      setIsFullscreen(!!document.fullscreenElement);
    };
    document.addEventListener("fullscreenchange", handleChange);
    return () => document.removeEventListener("fullscreenchange", handleChange);
  }, []);

  // Blob URL cleanup on unmount or src change.
  useEffect(() => {
    return () => URL.revokeObjectURL(src);
  }, [src]);

  // Auto-hide controls after 2.5s of no mouse movement
  const resetHideTimer = useCallback(() => {
    setShowControls(true);
    if (hideTimerRef.current) clearTimeout(hideTimerRef.current);
    hideTimerRef.current = setTimeout(() => setShowControls(false), 2500);
  }, []);

  useEffect(() => {
    resetHideTimer();
    return () => {
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current);
    };
  }, [resetHideTimer]);

  // Listen for iframe messages (ready, diagnostic, error)
  useEffect(() => {
    const handleMessage = (e: MessageEvent) => {
      if (e.data?.source !== "aniya-preview") return;
      if (e.data?.type === "ready") {
        setIframeReady(true);
      }
      if (e.data?.type === "diagnostic") {
        // diagnostic info available via e.data.diagnostic
      }
      if (e.data?.type === "error") {
        // preview errors available via e.data.errors
      }
    };
    window.addEventListener("message", handleMessage);
    return () => window.removeEventListener("message", handleMessage);
  }, []);

  // Reset ready state when src changes
  useEffect(() => {
    setIframeReady(false);
  }, [src]);

  // Escape key to close (capture phase so it fires before iframe gets it)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      }
      if (e.key === "f" || e.key === "F") {
        e.preventDefault();
        toggleFullscreen();
      }
    };
    window.addEventListener("keydown", handleKeyDown, true);
    return () => window.removeEventListener("keydown", handleKeyDown, true);
  }, [onClose, toggleFullscreen]);

  if (error) {
    return (
      <Portal>
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
          className="fixed inset-0 bg-black flex items-center justify-center"
          style={{ zIndex: Z_INDEX.FULLSCREEN }}
        >
          <button
            onClick={onClose}
            className="absolute top-4 right-4 p-2 rounded-lg text-white/40 hover:text-white hover:bg-white/10 transition-colors"
          >
            <X size={20} />
          </button>
          <p className="text-red-400 text-sm max-w-md text-center px-6">{error}</p>
        </motion.div>
      </Portal>
    );
  }

  return (
    <Portal>
    <motion.div
      ref={containerRef}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.15 }}
      className="fixed inset-0 bg-black select-none"
      style={{ zIndex: Z_INDEX.FULLSCREEN }}
      onMouseMove={resetHideTimer}
      onMouseEnter={() => setShowControls(true)}
    >
      {/* Auto-hiding top bar */}
      <motion.div
        initial={false}
        animate={{ opacity: showControls ? 1 : 0 }}
        transition={{ duration: 0.2 }}
        className="absolute top-0 left-0 right-0 z-10 flex items-center justify-between px-4 py-3 pointer-events-none"
      >
        <span className="text-sm text-white/50 font-medium pointer-events-auto">
          {t.preview.preview}
        </span>
        <div className="flex items-center gap-1 pointer-events-auto">
          <button
            onClick={toggleFullscreen}
            className="p-2 rounded-lg text-white/50 hover:text-white hover:bg-white/10 transition-colors"
            title={isFullscreen ? t.preview.exitFullscreen : t.preview.fullscreen}
          >
            {isFullscreen ? <Minimize size={16} /> : <Maximize size={16} />}
          </button>
          <button
            onClick={onClose}
            className="p-2 rounded-lg text-white/50 hover:text-white hover:bg-white/10 transition-colors"
            title={t.preview.closePreview}
          >
            <X size={16} />
          </button>
        </div>
      </motion.div>

      {/* Loading spinner — shown until iframe signals ready */}
      {!iframeReady && (
        <div className="absolute inset-0 flex items-center justify-center bg-black z-20">
          <div className="w-6 h-6 border-2 border-white/20 border-t-white/60 rounded-full animate-spin" />
        </div>
      )}

      {/* Iframe — fills container, scaleDeck() inside the HTML handles viewport adaptation */}
      <div className="absolute inset-0 flex items-center justify-center bg-black overflow-hidden">
        <iframe
          ref={iframeRef}
          title={t.preview.htmlPreview}
          sandbox="allow-scripts"
          src={src}
          onLoad={() => {
            setTimeout(() => setIframeReady(true), 1000);
          }}
          style={{
            width: '100%',
            height: '100%',
            border: 'none',
          }}
        />
      </div>
    </motion.div>
    </Portal>
  );
}
