"use client"

import { useState } from "react"
import { authClient } from "@/lib/auth-client"

export default function SignIn() {
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  return (
    <form onSubmit={(e) => { e.preventDefault(); authClient.signIn.email({ email, password, callbackURL: "/dashboard" }) }}>
      <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
      <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
      <button>Sign in</button>
    </form>
  )
}
