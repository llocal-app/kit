import { useRouter } from "next/router";
import { useEffect, useState } from "react";

export default function TaskPage() {
  const { query } = useRouter();
  const [task, setTask] = useState<{ title: string; done: boolean } | null>(null);
  useEffect(() => {
    if (query.id) fetch(`/api/tasks/${query.id}`).then((r) => r.json()).then(setTask);
  }, [query.id]);
  return <h1>{task?.title}</h1>;
}
