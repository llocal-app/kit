import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { supabase } from "@/integrations/supabase/client";

const Auth = () => {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [signUp, setSignUp] = useState(false);
  const navigate = useNavigate();

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const { error } = signUp
      ? await supabase.auth.signUp({ email, password, options: { emailRedirectTo: `${window.location.origin}/` } })
      : await supabase.auth.signInWithPassword({ email, password });
    if (!error) navigate("/");
  };

  return (
    <form onSubmit={submit}>
      <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
      <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
      <button>{signUp ? "Sign up" : "Sign in"}</button>
      <button type="button" onClick={() => setSignUp(!signUp)}>Switch</button>
    </form>
  );
};

export default Auth;
