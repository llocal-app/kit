import { integer, pgTable, serial, text, timestamp } from "drizzle-orm/pg-core";

export const customers = pgTable("customers", {
  id: serial("id").primaryKey(),
  name: text("name").notNull(),
  phone: text("phone"),
});

export const orders = pgTable("orders", {
  id: serial("id").primaryKey(),
  customerId: integer("customer_id").references(() => customers.id),
  part: text("part").notNull(),
  createdBy: text("created_by"),
  createdAt: timestamp("created_at").defaultNow(),
});
