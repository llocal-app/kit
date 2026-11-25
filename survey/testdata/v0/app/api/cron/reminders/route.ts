import { gte } from "drizzle-orm"
import { db } from "@/lib/db"
import { bookings } from "@/lib/db/schema"

export async function GET(req: Request) {
  if (req.headers.get("authorization") !== `Bearer ${process.env.CRON_SECRET}`) {
    return new Response("Unauthorized", { status: 401 })
  }
  const soon = await db.select().from(bookings).where(gte(bookings.startsAt, new Date()))
  return Response.json({ reminded: soon.length })
}
