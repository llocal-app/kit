import express, { type Express } from "express";
import path from "path";
import type { Server } from "http";

export async function setupVite(app: Express, server: Server) {
  const { createServer } = await import("vite");
  const vite = await createServer({ server: { middlewareMode: true, hmr: { server } }, appType: "custom" });
  app.use(vite.middlewares);
}

export function serveStatic(app: Express) {
  const dist = path.resolve(import.meta.dirname, "public");
  app.use(express.static(dist));
  app.use("*", (_req, res) => res.sendFile(path.resolve(dist, "index.html")));
}
