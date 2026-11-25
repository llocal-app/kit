import { useParams } from "react-router-dom";
import { supabase } from "@/integrations/supabase/client";

const Job = () => {
  const { id } = useParams();

  const addPhoto = async (file: File) => {
    await supabase.storage.from("job-photos").upload(`${id}/${file.name}`, file);
  };

  const sendQuote = async () => {
    await supabase.functions.invoke("send-quote", { body: { jobId: id } });
    await supabase.from("jobs").update({ status: "quoted" }).eq("id", id);
  };

  return (
    <div>
      <input type="file" accept="image/*" onChange={(e) => e.target.files && addPhoto(e.target.files[0])} />
      <button onClick={sendQuote}>Send quote</button>
    </div>
  );
};

export default Job;
