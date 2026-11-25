import { createFileRoute, Link } from "@tanstack/react-router";

export const Route = createFileRoute("/")({
  component: () => (
    <main>
      <h1>Invoices</h1>
      <Link to="/auth">Sign in</Link>
    </main>
  ),
});
