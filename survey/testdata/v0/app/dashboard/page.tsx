"use client"

import { createBooking } from "@/app/actions"

export default function Dashboard() {
  return (
    <form action={createBooking}>
      <input name="customer" placeholder="Customer" />
      <input name="startsAt" type="datetime-local" />
      <button>Book</button>
    </form>
  )
}
