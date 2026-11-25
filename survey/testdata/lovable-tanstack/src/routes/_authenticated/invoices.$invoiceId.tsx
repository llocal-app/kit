import { createFileRoute } from "@tanstack/react-router";
import { supabase } from "@/integrations/supabase/client";

export const Route = createFileRoute("/_authenticated/invoices/$invoiceId")({
  loader: async ({ params }) => {
    const { data } = await supabase.from("invoices").select("*").eq("id", params.invoiceId).single();
    return data;
  },
  component: () => <pre>{JSON.stringify(Route.useLoaderData(), null, 2)}</pre>,
});
