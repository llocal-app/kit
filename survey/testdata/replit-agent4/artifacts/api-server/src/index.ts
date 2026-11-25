import express from "express";
import { clerkMiddleware } from "@clerk/express";
import router from "./routes";

const app = express();
app.use(express.json());
app.use(clerkMiddleware());
app.use("/api", router);

app.listen(Number(process.env.PORT ?? 8080), "0.0.0.0");
