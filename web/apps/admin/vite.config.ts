import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  base: "/admin-ui/",
  plugins: [react()],
  server: {
    port: 9512,
    proxy: {
      "/api": "http://localhost:9505"
    }
  }
});
