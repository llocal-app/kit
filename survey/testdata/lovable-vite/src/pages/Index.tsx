import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { supabase } from "@/integrations/supabase/client";

const Index = () => {
  const { data: jobs } = useQuery({
    queryKey: ["jobs"],
    queryFn: async () => {
      const { data, error } = await supabase.from("jobs").select("*").order("created_at", { ascending: false });
      if (error) throw error;
      return data;
    },
  });

  return (
    <ul>
      {jobs?.map((j) => (
        <li key={j.id}><Link to={`/jobs/${j.id}`}>{j.customer}</Link> {j.status}</li>
      ))}
    </ul>
  );
};

export default Index;
