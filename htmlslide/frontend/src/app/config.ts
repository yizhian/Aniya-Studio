export const APP_CONFIG = {
  routes: {
    home: "/",
    editor: "/editor",
  },
  editor: {
    viewport: {
      width: 1920,
      height: 1080,
      paddingX: 80,
      paddingY: 80,
      maxScale: 1,
    },
    animation: {
      panelDuration: 0.2,
      homeIntroDuration: 0.6,
      navigateDelayMs: 1200,
      updateScaleDelayMs: 50,
    },
  },
  interaction: {
    submitKey: "Enter",
    htmlFileAccept: ".md,.txt,.pdf,.docx,.png,.jpg,.jpeg,.gif,.webp,.svg,.html,text/html",
    agentgoBaseUrl: import.meta.env.VITE_AGENTGO_BASE_URL || "http://localhost:8080",
  },
} as const;
