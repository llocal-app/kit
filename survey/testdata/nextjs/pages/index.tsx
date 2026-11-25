import { useEffect, useState } from "react";
import Link from "next/link";
import { signIn, useSession } from "next-auth/react";

export default function Home() {
  const { data: session } = useSession();
  const [tasks, setTasks] = useState<{ id: number; title: string }[]>([]);
  useEffect(() => {
    if (session) fetch("/api/tasks").then((r) => r.json()).then(setTasks);
  }, [session]);
  if (!session) return <button onClick={() => signIn("github")}>Sign in</button>;
  return <ul>{tasks.map((t) => <li key={t.id}><Link href={`/tasks/${t.id}`}>{t.title}</Link></li>)}</ul>;
}
