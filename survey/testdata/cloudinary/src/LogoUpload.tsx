const uploadURL = "https://api.cloudinary.com/v1_1/demo-yard/image/upload";

export function LogoUpload({ onUploaded }: { onUploaded: (url: string) => void }) {
  async function upload(file: File) {
    const form = new FormData();
    form.append("file", file);
    form.append("upload_preset", "letterhead_logos");
    const res = await fetch(uploadURL, { method: "POST", body: form });
    onUploaded((await res.json()).secure_url);
  }
  return <input type="file" accept="image/*" onChange={(e) => e.target.files && upload(e.target.files[0])} />;
}
