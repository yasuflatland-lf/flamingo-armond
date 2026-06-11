import type en from "../../messages/en.json";
import type { Locale } from "./config";

declare module "next-intl" {
  interface AppConfig {
    Messages: typeof en;
    Locale: Locale;
  }
}
