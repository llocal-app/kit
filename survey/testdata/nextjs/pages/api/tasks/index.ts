import type { NextApiRequest, NextApiResponse } from "next";
import { getServerSession } from "next-auth/next";
import { authOptions } from "../auth/[...nextauth]";
import { prisma } from "../../../lib/prisma";

export default async function handler(req: NextApiRequest, res: NextApiResponse) {
  const session = await getServerSession(req, res, authOptions);
  if (!session?.user?.email) return res.status(401).end();
  const owner = await prisma.user.findUniqueOrThrow({ where: { email: session.user.email } });
  if (req.method === "POST") {
    return res.json(await prisma.task.create({ data: { title: req.body.title, ownerId: owner.id } }));
  }
  res.json(await prisma.task.findMany({ where: { ownerId: owner.id } }));
}
