import { Router } from "express";
import { getAuth, requireAuth } from "@clerk/express";
import { db, orders } from "@workspace/db";

const router = Router();

router.get("/healthz", (_req, res) => res.json({ ok: true }));

router.get("/orders", requireAuth(), async (_req, res) => {
  res.json(await db.select().from(orders));
});

router.post("/orders", requireAuth(), async (req, res) => {
  const { userId } = getAuth(req);
  const [order] = await db.insert(orders).values({ ...req.body, createdBy: userId }).returning();
  res.status(201).json(order);
});

export default router;
