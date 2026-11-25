import express from "express";
import { registerRoutes } from "./routes";
import { serveStatic, setupVite } from "./vite";

const app = express();
app.use(express.json());

(async () => {
  const server = await registerRoutes(app);
  if (app.get("env") === "development") {
    await setupVite(app, server);
  } else {
    serveStatic(app);
  }
  const port = parseInt(process.env.PORT || "5000", 10);
  server.listen({ port, host: "0.0.0.0", reusePort: true }, () => console.log(`serving on port ${port}`));
})();
