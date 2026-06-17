import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// base: './' emits relative asset URLs so the build works under the GitHub
// Pages project sub-path (https://<user>.github.io/flamingo-armond/) without
// hardcoding the repository name.
export default defineConfig({
  base: "./",
  plugins: [react()],
});
