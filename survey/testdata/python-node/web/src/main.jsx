import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";

function Visitors() {
  const [visitors, setVisitors] = useState([]);
  useEffect(() => {
    fetch("/api/visitors").then((r) => r.json()).then(setVisitors);
  }, []);
  return <ul>{visitors.map((v) => <li key={v.id}>{v.name}, here for {v.host}</li>)}</ul>;
}

createRoot(document.getElementById("root")).render(<Visitors />);
