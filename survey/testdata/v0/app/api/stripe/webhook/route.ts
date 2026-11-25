import Stripe from "stripe"
import { eq } from "drizzle-orm"
import { db } from "@/lib/db"
import { bookings } from "@/lib/db/schema"

const stripe = new Stripe(process.env.STRIPE_SECRET_KEY!)

export async function POST(req: Request) {
  const event = stripe.webhooks.constructEvent(
    await req.text(),
    req.headers.get("stripe-signature")!,
    process.env.STRIPE_WEBHOOK_SECRET!,
  )
  if (event.type === "checkout.session.completed") {
    await db.update(bookings).set({ paid: true }).where(eq(bookings.id, Number(event.data.object.metadata?.bookingId)))
  }
  return Response.json({ received: true })
}
