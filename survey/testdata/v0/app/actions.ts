"use server"

import { headers } from "next/headers"
import { revalidatePath } from "next/cache"
import { auth } from "@/lib/auth"
import { db } from "@/lib/db"
import { bookings } from "@/lib/db/schema"

export async function createBooking(form: FormData) {
  const session = await auth.api.getSession({ headers: await headers() })
  if (!session) throw new Error("Not signed in")
  await db.insert(bookings).values({
    customer: String(form.get("customer")),
    startsAt: new Date(String(form.get("startsAt"))),
    userId: session.user.id,
  })
  revalidatePath("/")
}
