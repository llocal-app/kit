import { createFileRoute } from "@tanstack/react-router";
import Stripe from "stripe";
import { supabaseAdmin } from "@/integrations/supabase/client.server";

const stripe = new Stripe(process.env.STRIPE_SECRET_KEY!);

export const Route = createFileRoute("/api/public/stripe-webhook")({
  server: {
    handlers: {
      POST: async ({ request }) => {
        const event = stripe.webhooks.constructEvent(
          await request.text(),
          request.headers.get("stripe-signature")!,
          process.env.STRIPE_WEBHOOK_SECRET!,
        );
        if (event.type === "checkout.session.completed") {
          await supabaseAdmin.from("invoices").update({ status: "paid" }).eq("id", event.data.object.client_reference_id!);
        }
        return new Response("ok");
      },
    },
  },
});
