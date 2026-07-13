import zh from "./zh";
import en from "./en";

export const Locale = {
  ZhCN: "zh-CN",
  EnUS: "en-US",
} as const;

export type LocaleValue = (typeof Locale)[keyof typeof Locale];

export const translations: Record<LocaleValue, typeof zh> = {
  "zh-CN": zh,
  "en-US": en,
};
