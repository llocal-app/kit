import type { Express } from "express";
import { createServer, type Server } from "http";
import { eq } from "drizzle-orm";
import { db } from "./db";
import { tools, users } from "@shared/schema";
import { isAuthenticated, setupAuth } from "./replitAuth";

export async function registerRoutes(app: Express): Promise<Server> {
  await setupAuth(app);

  app.get("/api/auth/user", isAuthenticated, async (req: any, res) => {
    const [user] = await db.select().from(users).where(eq(users.id, req.user.claims.sub));
    res.json(user);
  });

  app.get("/api/tools", isAuthenticated, async (_req, res) => {
    res.json(await db.select().from(tools));
  });

  app.post("/api/tools", isAuthenticated, async (req, res) => {
    const [tool] = await db.insert(tools).values({ name: req.body.name }).returning();
    res.status(201).json(tool);
  });

  app.patch("/api/tools/:id/checkout", isAuthenticated, async (req: any, res) => {
    const [tool] = await db.update(tools)
      .set({ checkedOutBy: req.user.claims.sub, checkedOutAt: new Date() })
      .where(eq(tools.id, Number(req.params.id))).returning();
    res.json(tool);
  });

  app.delete("/api/tools/:id", isAuthenticated, async (req, res) => {
    await db.delete(tools).where(eq(tools.id, Number(req.params.id)));
    res.status(204).end();
  });

  return createServer(app);
}
