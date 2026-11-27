import type { CapacitorConfig } from "@capacitor/cli";

const config: CapacitorConfig = {
  appId: "io.github.axuitomo.cfstgui",
  appName: "CFST-GUI",
  webDir: "dist",
  android: {
    path: "../../apps/android",
  },
};

export default config;
