import React from "react";
import { loaderStyles } from "./LoadingOverlay.css";

interface LoadingOverlayProps {
  variant?: "plain" | "frosted";
  themeMode?: "light" | "dark";
}

const LOADING_TEXT = "Loading";

let styleInjected = false;

const lightVars = {
  "--text-color": "#1a1a1a",
  "--shadow-color": "#888888",
  "--shine-color": "#00000015",
} as React.CSSProperties;

const darkVars = {
  "--text-color": "#ffffff",
  "--shadow-color": "#aaaaaa",
  "--shine-color": "#ffffff40",
} as React.CSSProperties;

export const LoadingOverlay: React.FC<LoadingOverlayProps> = ({
  variant = "plain",
  themeMode = "dark",
}) => {
  React.useEffect(() => {
    if (styleInjected) return;
    styleInjected = true;
    const style = document.createElement("style");
    style.textContent = loaderStyles;
    document.head.appendChild(style);
    return () => {
      styleInjected = false;
      style.remove();
    };
  }, []);

  const isFrosted = variant === "frosted";
  const vars = themeMode === "light" ? lightVars : darkVars;

  return (
    <div className={`loading-overlay${isFrosted ? " is-frosted" : ""}`}>
      <div
        className="absolute inset-0 z-30 flex items-center justify-center"
        style={{
          background: isFrosted
            ? "rgba(255, 255, 255, 0.12)"
            : "transparent",
          backdropFilter: isFrosted ? "blur(3px)" : "none",
        }}
      >
        <div className="loader" style={vars}>
          <div className="text"><span>{LOADING_TEXT}</span></div>
          <div className="text"><span>{LOADING_TEXT}</span></div>
          <div className="text"><span>{LOADING_TEXT}</span></div>
          <div className="text"><span>{LOADING_TEXT}</span></div>
          <div className="text"><span>{LOADING_TEXT}</span></div>
          <div className="text"><span>{LOADING_TEXT}</span></div>
          <div className="text"><span>{LOADING_TEXT}</span></div>
          <div className="text"><span>{LOADING_TEXT}</span></div>
          <div className="text"><span>{LOADING_TEXT}</span></div>
          <div className="line" />
        </div>
      </div>
    </div>
  );
};

export default LoadingOverlay;
