import { desc } from "drizzle-orm"
import { db } from "@/lib/db"
import { bookings } from "@/lib/db/schema"

export default async function Page() {
  const upcoming = await db.select().from(bookings).orderBy(desc(bookings.startsAt)).limit(20)
  return (
    <main>
      <h1>Upcoming bookings</h1>
      <ul>{upcoming.map((b) => <li key={b.id}>{b.customer}</li>)}</ul>
    </main>
  )
}
