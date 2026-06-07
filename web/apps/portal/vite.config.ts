import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  base: "/portal/",
  plugins: [react()],
  server: {
    port: 9511,
    proxy: {
      "/api": "http://localhost:9505"
    }
  }
});
