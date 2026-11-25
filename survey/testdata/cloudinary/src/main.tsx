import { useState } from "react";
import { createRoot } from "react-dom/client";
import { LogoUpload } from "./LogoUpload";

function Letterhead() {
  const [logo, setLogo] = useState("");
  return (
    <main>
      {logo && <img src={logo} alt="Company logo" height={80} />}
      <LogoUpload onUploaded={setLogo} />
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<Letterhead />);
