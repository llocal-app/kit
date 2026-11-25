import { boolean, integer, pgTable, serial, text, timestamp } from "drizzle-orm/pg-core"

export const user = pgTable("user", {
  id: text("id").primaryKey(),
  name: text("name").notNull(),
  email: text("email").notNull().unique(),
  createdAt: timestamp("created_at").notNull(),
})

export const session = pgTable("session", {
  id: text("id").primaryKey(),
  token: text("token").notNull().unique(),
  userId: text("user_id").notNull().references(() => user.id),
  expiresAt: timestamp("expires_at").notNull(),
})

export const bookings = pgTable("bookings", {
  id: serial("id").primaryKey(),
  userId: text("user_id").references(() => user.id),
  customer: text("customer").notNull(),
  startsAt: timestamp("starts_at").notNull(),
  seats: integer("seats").default(1),
  paid: boolean("paid").default(false),
})
