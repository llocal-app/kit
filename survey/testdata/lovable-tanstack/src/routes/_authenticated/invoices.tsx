import { createFileRoute } from "@tanstack/react-router";
import { useServerFn } from "@tanstack/react-start";
import { supabase } from "@/integrations/supabase/client";
import { sendInvoice } from "@/lib/invoices.functions";

export const Route = createFileRoute("/_authenticated/invoices")({
  loader: async () => {
    const { data } = await supabase.from("invoices").select("*, customers(name)");
    return data ?? [];
  },
  component: Invoices,
});

function Invoices() {
  const invoices = Route.useLoaderData();
  const send = useServerFn(sendInvoice);
  return (
    <ul>
      {invoices.map((i) => (
        <li key={i.id}>{i.customers?.name} <button onClick={() => send({ data: { id: i.id } })}>Send</button></li>
      ))}
    </ul>
  );
}
