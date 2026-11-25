import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { createLovableAuth } from "@lovable.dev/cloud-auth-js";
import { supabase } from "@/integrations/supabase/client";

const lovable = createLovableAuth();

export const Route = createFileRoute("/auth")({ component: Auth });

function Auth() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const navigate = useNavigate();
  return (
    <form onSubmit={async (e) => {
      e.preventDefault();
      const { error } = await supabase.auth.signInWithPassword({ email, password });
      if (!error) navigate({ to: "/invoices" });
    }}>
      <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
      <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
      <button>Sign in</button>
      <button type="button" onClick={() => lovable.auth.signInWithOAuth("google")}>Continue with Google</button>
    </form>
  );
}
